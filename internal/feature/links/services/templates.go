package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	linksports "svc-registry/internal/feature/links/ports"
	"svc-registry/internal/feature/links/repositories"
	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/ptr"
)

func (s *TemplatesService) ListLinkTemplates(ctx context.Context, p accesstypes.Principal, id uuid.UUID) ([]linkstypes.Template, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id); err != nil {
		return nil, err
	}
	items, err := s.templateRepository.Effective(ctx, id)
	return items, apperr.Wrap(err)
}

func (s *TemplatesService) PutLinkTemplate(ctx context.Context, p accesstypes.Principal, id uuid.UUID, linkKey string, in linkstypes.PutTemplate) (linkstypes.Template, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return linkstypes.Template{}, err
	}
	t, err := linkstypes.ValidateTemplate(id, linkKey, in)
	if err != nil {
		return linkstypes.Template{}, apperr.Wrap(err)
	}
	if t.IconFile != nil && (s.icons == nil || !s.icons.Exists(*t.IconFile)) {
		return linkstypes.Template{}, apperr.Wrap(linkstypes.InvalidTemplateIcon)
	}
	saved, err := s.templateRepository.Put(ctx, t)
	return saved, apperr.Wrap(err)
}

func (s *TemplatesService) DeleteLinkTemplate(ctx context.Context, p accesstypes.Principal, id uuid.UUID, rawKey string) error {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return err
	}
	key, err := linkstypes.ValidateKey(rawKey)
	if err != nil {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(s.templateRepository.Delete(ctx, id, key))
}

func (s *TemplatesService) ListNodeVars(ctx context.Context, p accesstypes.Principal, id uuid.UUID) ([]linkstypes.Var, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id); err != nil {
		return nil, err
	}
	items, err := s.templateRepository.EffectiveVars(ctx, id)
	return items, apperr.Wrap(err)
}

func (s *TemplatesService) ReplaceNodeVars(ctx context.Context, p accesstypes.Principal, id uuid.UUID, raw map[string]string) ([]linkstypes.Var, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return nil, err
	}
	vars, err := linkstypes.ValidateVars(raw)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if err := s.templateRepository.ReplaceVars(ctx, id, vars); err != nil {
		return nil, apperr.Wrap(err)
	}
	items, err := s.templateRepository.EffectiveVars(ctx, id)
	return items, apperr.Wrap(err)
}

func (s *TemplatesService) PreviewLinkTemplate(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in linkstypes.Preview) ([]linkstypes.Link, error) {
	sc, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return nil, err
	}
	if err := linkstypes.CheckTemplate(in.Template); err != nil {
		return nil, apperr.Wrap(err)
	}
	project := sc
	switch {
	case in.ProjectID != nil && *in.ProjectID != id:
		under, err := s.templateRepository.IsUnder(ctx, *in.ProjectID, id)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		if !under {
			return nil, apperr.New(apperr.NotFound)
		}
		if project, err = s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, *in.ProjectID); err != nil {
			return nil, apperr.New(apperr.NotFound)
		}
		if project.Node.Kind != catalogtypes.KindProject {
			return nil, apperr.New(apperr.NotFound)
		}
	case sc.Node.Kind != catalogtypes.KindProject:
		return nil, apperr.Wrap(linkstypes.InvalidProject)
	}
	q := linkstypes.LinkQuery{Environment: ptr.Value(in.Environment)}
	if in.Branch != nil {
		q.Branch = *in.Branch
	}
	c, _, err := s.projects.projectContext(ctx, project, q.Branch)
	if err != nil {
		return nil, err
	}
	template := in.Template
	preview := []linkstypes.Template{{NodeID: id, LinkKey: "preview", Template: &template}}
	out := linkstypes.Expand(c, preview, q.Environment)
	return out, s.projects.attachChecks(ctx, out)
}

type TemplatesService struct {
	catalog            *catalogservices.CatalogServices
	icons              linksports.IconFilesPort
	templateRepository *repositories.TemplateRepository
	projects           *ProjectsService
}
