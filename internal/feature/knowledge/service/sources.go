package service

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/forge"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) GetKnowledgeSource(ctx context.Context, p access.Principal, id uuid.UUID) (*knowledge.Source, error) {
	if _, err := s.knowledgeProject(ctx, p, catalog.PermRead, id); err != nil {
		return nil, err
	}
	src, err := s.sources.Get(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if src != nil {
		out, err := s.decorateSource(ctx, src.Source)
		out.Origin = knowledge.OriginOwn
		return &out, err
	}
	link, err := s.forge.ProjectLink(ctx, id)
	if err != nil || link == nil {
		return nil, err
	}
	out, err := s.decorateSource(ctx, knowledge.Source{Kind: knowledge.SourceRemote, Forge: string(link.Kind), URL: link.WebURL,
		APIURL: link.APIURL, Credentials: link.Credentials, UpdatedAt: link.SyncedAt, Heads: map[string]string{}})
	out.Origin = knowledge.OriginForge
	return &out, err
}

func (s *Service) decorateSource(ctx context.Context, src knowledge.Source) (knowledge.Source, error) {
	if src.Credentials.SecretID == nil {
		return src, nil
	}
	refs, err := s.catalog.SecretRefs(ctx, []uuid.UUID{*src.Credentials.SecretID})
	if err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
	}
	if ref, ok := refs[*src.Credentials.SecretID]; ok {
		src.Credentials.Secret = &ref
	}
	return src, nil
}

func (s *Service) validSource(in knowledge.SourceInput) (knowledge.ValidSource, error) {
	v, err := knowledge.ValidateSource(in)
	if err != nil {
		return knowledge.ValidSource{}, apperr.Wrap(err)
	}
	if v.IsLocal() {
		if err := knowledge.LocalPathAllowed(s.cfg.Knowledge.LocalRoots, v.Path); err != nil {
			return knowledge.ValidSource{}, apperr.Wrap(err)
		}
	}
	return v, nil
}

func (s *Service) prepareSource(ctx context.Context, id uuid.UUID, in knowledge.SourceInput, checkSecret bool) (knowledge.NewSource, error) {
	v, err := s.validSource(in)
	if err != nil {
		return knowledge.NewSource{}, err
	}
	if checkSecret {
		if err := s.checkSecret(ctx, id, v.SecretID); err != nil {
			return knowledge.NewSource{}, err
		}
	}
	return knowledge.NewSource(v), nil
}

func (s *Service) checkSecret(ctx context.Context, projectID uuid.UUID, secret *uuid.UUID) error {
	if secret == nil {
		return nil
	}
	_, err := s.catalog.SecretFor(ctx, &projectID, *secret)
	return err
}

func sourceRepo(s knowledge.Source) (f *catalog.Forge, url *string, ok bool) {
	if s.Kind != knowledge.SourceRemote {
		return nil, nil, false
	}
	f, ferr := catalog.ValidateForge(s.Forge)
	url, uerr := catalog.ValidateRepoURL(s.URL)
	return f, url, ferr == nil && uerr == nil && f != nil && url != nil
}

func (s *Service) PutKnowledgeSource(ctx context.Context, p access.Principal, id uuid.UUID, in knowledge.SourceInput) (knowledge.Source, error) {
	if _, err := s.knowledgeProject(ctx, p, catalog.PermWrite, id); err != nil {
		return knowledge.Source{}, err
	}
	managed, err := s.forge.Managed(ctx, []uuid.UUID{id})
	if err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
	}
	if managed[id] {
		return knowledge.Source{}, apperr.Wrap(catalog.ConflictManagedByForge)
	}
	n, err := s.prepareSource(ctx, id, in, true)
	if err != nil {
		return knowledge.Source{}, err
	}
	saved, err := s.sources.Put(ctx, id, n)
	if err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
	}
	if saved, err = s.decorateSource(ctx, saved); err != nil {
		return knowledge.Source{}, err
	}
	saved.Origin = knowledge.OriginOwn
	f, url, ok := sourceRepo(saved)
	if !ok {
		return saved, nil
	}
	changes := catalog.NodeChanges{Forge: catalog.Change[catalog.Forge]{Set: true, Value: f}, RepoURL: catalog.Change[string]{Set: true, Value: url}}
	if _, err := s.catalog.ChangeNode(ctx, id, changes); err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
	}
	return saved, nil
}

func (s *Service) AdoptRemote(ctx context.Context, projectID uuid.UUID, src forge.RemoteSource) error {
	own, err := s.sources.Get(ctx, projectID)
	if err != nil || own != nil {
		return err
	}
	in := knowledge.SourceInput{Kind: string(knowledge.SourceRemote), Forge: string(src.Kind), URL: src.WebURL, APIURL: src.APIURL,
		HasCredential: true, NoCredentials: src.SecretID == nil}
	if src.SecretID != nil {
		id := src.SecretID.String()
		in.SecretID = &id
	}
	v, err := knowledge.ValidateSource(in)
	if err != nil {
		return err
	}
	if v.SecretID != nil {
		if _, err := s.catalog.SecretFor(ctx, &projectID, *v.SecretID); err != nil {
			v.SecretID = nil
		}
	}
	_, err = s.sources.Put(ctx, projectID, knowledge.NewSource(v))
	return err
}

func (s *Service) SourceRepository(ctx context.Context, id uuid.UUID) (*forge.Repository, error) {
	r, err := s.sourceRepos.Find(ctx, id)
	return r, apperr.Wrap(err)
}

func (s *Service) SourceReadme(ctx context.Context, id uuid.UUID) (*forge.Readme, error) {
	r, err := s.sourceRepos.Readme(ctx, id)
	return r, apperr.Wrap(err)
}

func (s *Service) DropOwn(ctx context.Context, projectID uuid.UUID) error {
	return s.sources.Delete(ctx, projectID)
}

func (s *Service) DeleteKnowledgeSource(ctx context.Context, p access.Principal, id uuid.UUID) error {
	if _, err := s.knowledgeProject(ctx, p, catalog.PermWrite, id); err != nil {
		return err
	}
	return apperr.Wrap(s.sources.Delete(ctx, id))
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

func (s *Service) CheckKnowledgeSource(ctx context.Context, p access.Principal, id uuid.UUID, in knowledge.SourceInput) (SourceCheck, error) {
	sc, err := s.catalog.Authorize(ctx, p, catalog.PermWrite, id)
	if err != nil {
		return SourceCheck{}, err
	}
	project := sc.Node.Kind == catalog.KindProject
	settings, err := s.effectiveSettings(ctx, id)
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
		t, err := s.sourceToken(ctx, id, &knowledge.StoredSource{CredentialsSecretID: v.SecretID})
		if err != nil {
			code, _ := failureOf(err)
			return SourceCheck{ErrorCode: code}, nil
		}
		token = t
	case v.Keep && project:
		if stored, err := s.sources.Get(ctx, id); err == nil && stored != nil && stored.Kind == v.Kind && stored.URL == v.URL {
			t, err := s.sourceToken(ctx, id, stored)
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
		if !catalog.BranchFits(name) {
			delete(heads, name)
		}
	}
	if !catalog.BranchFits(def) {
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
