package services

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/feature/skills/usecases"
	"svc-registry/internal/platform/apperr"
)

type SourcesService struct {
	parts
	catalog *catalogservices.CatalogServices
	readers knowledgeports.ReaderFactoryPort
	limits  skillstypes.Limits
	sync    *usecases.SyncSourceUseCase
}

type SourceInput struct {
	Kind         string
	Forge        string
	URL          string
	APIURL       string
	Path         string
	WorkingTree  *bool
	SecretID     *string
	Branch       string
	Root         string
	IntervalSecs *int
	Status       string
	Versions     string
	TagPattern   string
}

func (s *SourcesService) folder(ctx context.Context, p accesstypes.Principal, id uuid.UUID, action accesstypes.Action) (skillstypes.Scope, error) {
	return placeOf(ctx, s.parts, p, &id, nil, action)
}

func (s *SourcesService) ListSources(ctx context.Context, p accesstypes.Principal, folder uuid.UUID) ([]skillstypes.Source, error) {
	chain, err := folderChain(ctx, s.parts, folder)
	if err != nil {
		return nil, err
	}
	if err := s.access.canRead(ctx, p, skillstypes.Scope{OrganizationID: chain[0].OrganizationID}); err != nil {
		return nil, err
	}
	out, err := s.sources.List(ctx, folder)
	return out, apperr.Wrap(err)
}

func (s *SourcesService) source(ctx context.Context, p accesstypes.Principal, id uuid.UUID, action *accesstypes.Action) (skillstypes.Source, error) {
	src, err := s.sources.Get(ctx, id)
	if err != nil {
		return skillstypes.Source{}, apperr.Wrap(err)
	}
	if src == nil {
		return skillstypes.Source{}, apperr.New(apperr.NotFound)
	}
	scope := skillstypes.Scope{OrganizationID: src.OrganizationID}
	if err := s.access.canRead(ctx, p, scope); err != nil {
		return skillstypes.Source{}, err
	}
	if action != nil {
		if err := s.access.canWrite(ctx, p, scope, *action); err != nil {
			return skillstypes.Source{}, err
		}
	}
	return *src, nil
}

func (s *SourcesService) GetSource(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (skillstypes.Source, error) {
	return s.source(ctx, p, id, nil)
}

func (s *SourcesService) CreateSource(ctx context.Context, p accesstypes.Principal, folder uuid.UUID, in SourceInput) (skillstypes.Source, error) {
	scope, err := s.folder(ctx, p, folder, accesstypes.ActionCreate)
	if err != nil {
		return skillstypes.Source{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return skillstypes.Source{}, apperr.Wrap(err)
	}
	return s.save(ctx, p, skillstypes.Source{ID: id, FolderID: folder, OrganizationID: scope.OrganizationID}, in)
}

func (s *SourcesService) UpdateSource(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in SourceInput) (skillstypes.Source, error) {
	src, err := s.source(ctx, p, id, new(accesstypes.ActionUpdate))
	if err != nil {
		return skillstypes.Source{}, err
	}
	return s.save(ctx, p, src, in)
}

func (s *SourcesService) save(ctx context.Context, p accesstypes.Principal, src skillstypes.Source, in SourceInput) (skillstypes.Source, error) {
	out, err := s.validate(ctx, src, in)
	if err != nil {
		return skillstypes.Source{}, apperr.Wrap(err)
	}
	if err := s.sources.Save(ctx, out); err != nil {
		return skillstypes.Source{}, apperr.Wrap(err)
	}
	return s.GetSource(ctx, p, src.ID)
}

func (s *SourcesService) validate(ctx context.Context, src skillstypes.Source, in SourceInput) (skillstypes.Source, error) {
	v, err := knowledgetypes.ValidateSource(knowledgetypes.SourceInput{Kind: in.Kind, Forge: in.Forge, URL: in.URL, APIURL: in.APIURL,
		Path: in.Path, SecretID: in.SecretID, WorkingTree: in.WorkingTree, HasCredential: in.SecretID != nil})
	if err != nil {
		return skillstypes.Source{}, err
	}
	if v.IsLocal() {
		if _, err := knowledgetypes.ResolveLocalPath(s.cfg.Knowledge.LocalRoots, v.Path); err != nil {
			return skillstypes.Source{}, err
		}
	}
	if v.SecretID != nil {
		if _, err := s.catalog.SecretResolver.SecretFor(ctx, src.OrganizationID, *v.SecretID); err != nil {
			return skillstypes.Source{}, err
		}
	}
	root, err := skillstypes.CleanRoot(in.Root)
	branch := strings.TrimSpace(in.Branch)
	status, statusOK := skillstypes.ParseStatus(defaultTo(in.Status, string(skillstypes.StatusPublished)))
	versions := defaultTo(in.Versions, skillstypes.VersionsBranch)
	pattern := defaultTo(in.TagPattern, skillstypes.DefaultTagPattern)
	if err != nil || len(branch) > 255 || !statusOK || (in.IntervalSecs != nil && (*in.IntervalSecs < 300 || *in.IntervalSecs > 86400)) ||
		(versions != skillstypes.VersionsBranch && versions != skillstypes.VersionsTags) ||
		(versions == skillstypes.VersionsTags && v.Kind == knowledgetypes.SourceLocalDir) {
		return skillstypes.Source{}, skillstypes.InvalidSource
	}
	if _, err := skillstypes.ParseTagPattern(pattern); err != nil {
		return skillstypes.Source{}, err
	}
	src.Kind, src.WorkingTree, src.SecretID = string(v.Kind), v.WorkingTree, v.SecretID
	src.Forge, src.URL, src.APIURL, src.Path = optional(v.Forge), optional(v.URL), optional(v.APIURL), optional(v.Path)
	src.Branch, src.Root, src.IntervalSecs, src.NewStatus, src.Versions, src.TagPattern = branch, root, in.IntervalSecs, status, versions, pattern
	return src, nil
}

func defaultTo(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (s *SourcesService) DeleteSource(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if _, err := s.source(ctx, p, id, new(accesstypes.ActionDelete)); err != nil {
		return err
	}
	return apperr.Wrap(s.sources.Delete(ctx, id))
}

func (s *SourcesService) QueueSource(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (skillstypes.Source, error) {
	if _, err := s.source(ctx, p, id, new(accesstypes.ActionUpdate)); err != nil {
		return skillstypes.Source{}, err
	}
	if err := s.sources.Queue(ctx, id); err != nil {
		return skillstypes.Source{}, apperr.Wrap(err)
	}
	return s.GetSource(ctx, p, id)
}

func (s *SourcesService) ClaimSources(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	out, err := s.sources.Claim(ctx, limit, leaseSecs)
	return out, apperr.Wrap(err)
}

func (s *SourcesService) ExtendSource(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(s.sources.Extend(ctx, id, leaseSecs))
}

func (s *SourcesService) RunSource(ctx context.Context, id uuid.UUID) error {
	src, err := s.sources.Get(ctx, id)
	if err != nil || src == nil {
		return apperr.Wrap(err)
	}
	started := time.Now().UTC()
	res := s.runSource(ctx, *src)
	res.Run.StartedAt, res.Run.FinishedAt = started.Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)
	interval := int(s.cfg.Skills.SyncIntervalSecs)
	if src.IntervalSecs != nil {
		interval = *src.IntervalSecs
	}
	return apperr.Wrap(s.sources.Finish(context.WithoutCancel(ctx), id, res.Run, res.Commit, res.Tags, interval, src.ConfigChangedAt))
}

func (s *SourcesService) runSource(ctx context.Context, src skillstypes.Source) usecases.SyncResult {
	fail := func(code string) usecases.SyncResult {
		return usecases.SyncResult{Run: skillstypes.Run{Status: skillstypes.RunFailed, Error: &code, Problems: []skillstypes.Problem{}}}
	}
	if s.store == nil {
		return fail(skillstypes.FailStorage)
	}
	token := ""
	if src.SecretID != nil {
		t, err := s.catalog.SecretResolver.ResolveSecret(ctx, *src.SecretID)
		if err != nil {
			return fail(skillstypes.FailUnauthorized)
		}
		token = t
	}
	reader, err := s.readers.Open(ctx, sourceOf(src), settingsOf(src), token)
	if err != nil {
		return fail(skillstypes.FailUnreachable)
	}
	return s.sync.Run(ctx, usecases.SyncInput{Source: src, Reader: reader, Author: authorOf(src), Policy: s.policy(ctx)})
}

func (s *SourcesService) policy(ctx context.Context) func(uuid.UUID) skillstypes.Effective {
	return func(id uuid.UUID) skillstypes.Effective {
		sk, err := s.skills.Get(ctx, id)
		if err != nil || sk == nil {
			return skillstypes.Merge(nil, nil)
		}
		pol, err := (&cardBuilder{parts: s.parts}).policies(ctx, []skillstypes.Skill{*sk})
		if err != nil {
			return skillstypes.Merge(nil, nil)
		}
		return pol.effective(*sk)
	}
}

func sourceOf(src skillstypes.Source) knowledgetypes.Source {
	out := knowledgetypes.Source{Kind: knowledgetypes.SourceKind(src.Kind), WorkingTree: src.WorkingTree}
	out.Forge, out.URL, out.APIURL, out.Path = deref(src.Forge), deref(src.URL), deref(src.APIURL), deref(src.Path)
	return out
}

func settingsOf(src skillstypes.Source) knowledgetypes.Settings {
	include := "**"
	if src.Root != "" {
		include = src.Root + "/**"
	}
	refs := []knowledgetypes.RefPattern{}
	if src.Branch != "" {
		refs = append(refs, knowledgetypes.RefPattern{Pattern: src.Branch, Kind: knowledgetypes.RefBranch})
	}
	if src.Versions == skillstypes.VersionsTags {
		refs = append(refs, knowledgetypes.RefPattern{Pattern: "**", Kind: knowledgetypes.RefTag})
	}
	return knowledgetypes.Settings{Include: []string{include}, Exclude: []string{}, Refs: refs}
}

func authorOf(src skillstypes.Source) string {
	if src.URL != nil {
		full := knowledgetypes.Source{URL: *src.URL}.FullPath()
		if full != "" {
			return full
		}
	}
	if src.Path != nil {
		return path.Base(*src.Path)
	}
	return ""
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
