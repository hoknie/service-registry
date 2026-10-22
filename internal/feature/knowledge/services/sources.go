package services

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgeservices "svc-registry/internal/feature/forge/services"
	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
)

func (s *SourcesService) GetKnowledgeSource(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (*knowledgetypes.Source, error) {
	if _, err := s.documents.knowledgeProject(ctx, p, catalogtypes.PermRead, id); err != nil {
		return nil, err
	}
	src, err := s.sourceRepository.Get(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if src != nil {
		out, err := s.decorateSource(ctx, src.Source)
		out.Origin = knowledgetypes.OriginOwn
		return &out, err
	}
	link, err := s.forge.Unlink.ProjectLink(ctx, id)
	if err != nil || link == nil {
		return nil, err
	}
	out, err := s.decorateSource(ctx, knowledgetypes.Source{Kind: knowledgetypes.SourceRemote, Forge: string(link.Kind), URL: link.WebURL,
		APIURL: link.APIURL, Credentials: link.Credentials, UpdatedAt: link.SyncedAt, Heads: map[string]string{}})
	out.Origin = knowledgetypes.OriginForge
	return &out, err
}

func (s *SourcesService) decorateSource(ctx context.Context, src knowledgetypes.Source) (knowledgetypes.Source, error) {
	if src.Credentials.SecretID == nil {
		return src, nil
	}
	refs, err := s.catalog.SecretResolver.SecretRefs(ctx, []uuid.UUID{*src.Credentials.SecretID})
	if err != nil {
		return knowledgetypes.Source{}, apperr.Wrap(err)
	}
	if ref, ok := refs[*src.Credentials.SecretID]; ok {
		src.Credentials.Secret = &ref
	}
	return src, nil
}

func (s *SourcesService) validSource(in knowledgetypes.SourceInput) (knowledgetypes.ValidSource, error) {
	v, err := knowledgetypes.ValidateSource(in)
	if err != nil {
		return knowledgetypes.ValidSource{}, apperr.Wrap(err)
	}
	if v.IsLocal() {
		if err := knowledgetypes.LocalPathAllowed(s.cfg.Knowledge.LocalRoots, v.Path); err != nil {
			return knowledgetypes.ValidSource{}, apperr.Wrap(err)
		}
	}
	return v, nil
}

func (s *SourcesService) prepareSource(ctx context.Context, id uuid.UUID, in knowledgetypes.SourceInput, checkSecret bool) (knowledgetypes.NewSource, error) {
	v, err := s.validSource(in)
	if err != nil {
		return knowledgetypes.NewSource{}, err
	}
	if checkSecret {
		if err := s.checkSecret(ctx, id, v.SecretID); err != nil {
			return knowledgetypes.NewSource{}, err
		}
	}
	return knowledgetypes.NewSource(v), nil
}

func (s *SourcesService) checkSecret(ctx context.Context, projectID uuid.UUID, secret *uuid.UUID) error {
	if secret == nil {
		return nil
	}
	_, err := s.catalog.SecretResolver.SecretFor(ctx, &projectID, *secret)
	return err
}

func sourceRepo(s knowledgetypes.Source) (f *catalogtypes.Forge, url *string, ok bool) {
	if s.Kind != knowledgetypes.SourceRemote {
		return nil, nil, false
	}
	f, ferr := catalogtypes.ValidateForge(s.Forge)
	url, uerr := catalogtypes.ValidateRepoURL(s.URL)
	return f, url, ferr == nil && uerr == nil && f != nil && url != nil
}

func (s *SourcesService) PutKnowledgeSource(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in knowledgetypes.SourceInput) (knowledgetypes.Source, error) {
	if _, err := s.documents.knowledgeProject(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return knowledgetypes.Source{}, err
	}
	managed, err := s.forge.Ports.Managed(ctx, []uuid.UUID{id})
	if err != nil {
		return knowledgetypes.Source{}, apperr.Wrap(err)
	}
	if managed[id] {
		return knowledgetypes.Source{}, apperr.Wrap(catalogtypes.ConflictManagedByForge)
	}
	n, err := s.prepareSource(ctx, id, in, true)
	if err != nil {
		return knowledgetypes.Source{}, err
	}
	saved, err := s.sourceRepository.Put(ctx, id, n)
	if err != nil {
		return knowledgetypes.Source{}, apperr.Wrap(err)
	}
	if saved, err = s.decorateSource(ctx, saved); err != nil {
		return knowledgetypes.Source{}, err
	}
	saved.Origin = knowledgetypes.OriginOwn
	f, url, ok := sourceRepo(saved)
	if !ok {
		return saved, nil
	}
	changes := catalogtypes.NodeChanges{Forge: catalogtypes.Change[catalogtypes.Forge]{Set: true, Value: f}, RepoURL: catalogtypes.Change[string]{Set: true, Value: url}}
	if _, err := s.catalog.System.ChangeNode(ctx, id, changes); err != nil {
		return knowledgetypes.Source{}, apperr.Wrap(err)
	}
	return saved, nil
}

func (s *SourcesService) AdoptRemote(ctx context.Context, projectID uuid.UUID, src forgetypes.RemoteSource) error {
	own, err := s.sourceRepository.Get(ctx, projectID)
	if err != nil || own != nil {
		return err
	}
	in := knowledgetypes.SourceInput{Kind: string(knowledgetypes.SourceRemote), Forge: string(src.Kind), URL: src.WebURL, APIURL: src.APIURL,
		HasCredential: true, NoCredentials: src.SecretID == nil}
	if src.SecretID != nil {
		id := src.SecretID.String()
		in.SecretID = &id
	}
	v, err := knowledgetypes.ValidateSource(in)
	if err != nil {
		return err
	}
	if v.SecretID != nil {
		if _, err := s.catalog.SecretResolver.SecretFor(ctx, &projectID, *v.SecretID); err != nil {
			v.SecretID = nil
		}
	}
	_, err = s.sourceRepository.Put(ctx, projectID, knowledgetypes.NewSource(v))
	return err
}

func (s *SourcesService) SourceRepository(ctx context.Context, id uuid.UUID) (*forgetypes.Repository, error) {
	r, err := s.sourceRepoRepository.Find(ctx, id)
	return r, apperr.Wrap(err)
}

func (s *SourcesService) SourceReadme(ctx context.Context, id uuid.UUID) (*forgetypes.Readme, error) {
	r, err := s.sourceRepoRepository.Readme(ctx, id)
	return r, apperr.Wrap(err)
}

func (s *SourcesService) DropOwn(ctx context.Context, projectID uuid.UUID) error {
	return s.sourceRepository.Delete(ctx, projectID)
}

func (s *SourcesService) DeleteKnowledgeSource(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if _, err := s.documents.knowledgeProject(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return err
	}
	return apperr.Wrap(s.sourceRepository.Delete(ctx, id))
}

type SourceCheck struct {
	OK            bool
	Branches      int
	BranchNames   []string
	DefaultBranch string
	APIURL        string
	ErrorCode     string
}

const checkBranchNames = 200

func (s *SourcesService) CheckKnowledgeSource(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in knowledgetypes.SourceInput) (SourceCheck, error) {
	sc, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, id)
	if err != nil {
		return SourceCheck{}, err
	}
	project := sc.Node.Kind == catalogtypes.KindProject
	settings, err := s.settingsService.effectiveSettings(ctx, id)
	if err != nil {
		return SourceCheck{}, err
	}
	v, err := s.validSource(in)
	if err != nil {
		return SourceCheck{}, err
	}
	token := ""
	switch {
	case v.SecretID != nil:
		if err := s.checkSecret(ctx, id, v.SecretID); err != nil {
			return SourceCheck{}, err
		}
		t, err := s.collect.sourceToken(ctx, id, &knowledgetypes.StoredSource{CredentialsSecretID: v.SecretID})
		if err != nil {
			code, _ := failureOf(err)
			return SourceCheck{ErrorCode: code}, nil
		}
		token = t
	case v.Keep && project:
		if stored, err := s.sourceRepository.Get(ctx, id); err == nil && stored != nil && stored.Kind == v.Kind && stored.URL == v.URL {
			t, err := s.collect.sourceToken(ctx, id, stored)
			if err != nil {
				code, _ := failureOf(err)
				return SourceCheck{ErrorCode: code}, nil
			}
			token = t
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.Outbound.TimeoutSecs)*time.Second)
	defer cancel()
	settings.Branches = []string{"**"}
	reader, err := s.readers.Open(ctx, v.Source, settings, token)
	if err != nil {
		code, _ := failureOf(err)
		return SourceCheck{ErrorCode: code}, nil
	}
	heads, def, err := reader.Heads(ctx)
	if err != nil {
		code, _ := failureOf(err)
		return SourceCheck{ErrorCode: code}, nil
	}
	for name := range heads {
		if !catalogtypes.BranchFits(name) {
			delete(heads, name)
		}
	}
	if !catalogtypes.BranchFits(def) {
		def = ""
	}
	return SourceCheck{OK: true, Branches: len(heads), BranchNames: branchNames(heads, def), DefaultBranch: def, APIURL: v.APIURL}, nil
}

func branchNames(heads map[string]string, def string) []string {
	names := make([]string, 0, len(heads))
	for name := range heads {
		if name != def {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	if _, ok := heads[def]; ok && def != "" {
		names = append([]string{def}, names...)
	}
	if len(names) > checkBranchNames {
		names = names[:checkBranchNames]
	}
	return names
}

type SourcesService struct {
	catalog              *catalogservices.CatalogServices
	cfg                  *config.Config
	forge                *forgeservices.ForgeServices
	readers              knowledgeports.ReaderFactoryPort
	sourceRepoRepository *repositories.SourceRepoRepository
	sourceRepository     *repositories.SourceRepository
	collect              *CollectService
	documents            *DocumentsService
	settingsService      *SettingsService
}
