package services

import (
	"context"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgeports "svc-registry/internal/feature/forge/ports"
	"svc-registry/internal/feature/forge/repositories"
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

type ExclusionView struct {
	Exclusion forgetypes.Exclusion
	Project   *ExcludedProject
}

type ExcludedProject struct {
	ID   uuid.UUID
	Name string
	Path string
}

func (s *UnlinkService) ProjectLink(ctx context.Context, id uuid.UUID) (*forgetypes.ProjectLink, error) {
	l, err := s.repoRepository.Link(ctx, id)
	return l, apperr.Wrap(err)
}

func (s *UnlinkService) UnlinkProject(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := s.catalog.Branches.AuthorizeProject(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return err
	}
	link, err := s.repoRepository.Link(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if link == nil {
		return apperr.Wrap(forgetypes.ConflictNotLinked)
	}
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, link.NodeID); err != nil {
		return err
	}
	return apperr.Wrap(s.db.InTx(ctx, func(ctx context.Context) error {
		u, err := s.repoRepository.Unlink(ctx, id)
		if err != nil {
			return err
		}
		if u == nil {
			return forgetypes.ConflictNotLinked
		}
		if err := s.exclusionRepository.Insert(ctx, *u); err != nil {
			return err
		}
		src := forgetypes.RemoteSource{Kind: link.Kind, WebURL: link.WebURL, APIURL: link.APIURL}
		if link.Credentials.Kind == catalogtypes.CredentialsSecret {
			src.SecretID = link.Credentials.SecretID
		}
		return s.sources.AdoptRemote(ctx, id, src)
	}))
}

func (s *UnlinkService) ListExclusions(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) ([]ExclusionView, error) {
	if _, err := s.connectionsService.authorizeConnection(ctx, p, catalogtypes.PermRead, nodeID, id); err != nil {
		return nil, err
	}
	list, err := s.exclusionRepository.List(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make([]ExclusionView, 0, len(list))
	for _, e := range list {
		v := ExclusionView{Exclusion: e}
		if e.ProjectID != nil {
			if sc, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, *e.ProjectID); err == nil {
				slugs := make([]string, 0, len(sc.Path)+1)
				for _, n := range sc.Path {
					slugs = append(slugs, n.Node.Slug)
				}
				slugs = append(slugs, sc.Node.Slug)
				v.Project = &ExcludedProject{ID: sc.Node.ID, Name: sc.Node.Name, Path: strings.Join(slugs, "/")}
			}
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *UnlinkService) DeleteExclusion(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID, externalID string) error {
	if _, err := s.connectionsService.authorizeConnection(ctx, p, catalogtypes.PermAccess, nodeID, id); err != nil {
		return err
	}
	return apperr.Wrap(s.db.InTx(ctx, func(ctx context.Context) error {
		e, err := s.exclusionRepository.Delete(ctx, id, externalID)
		if err != nil {
			return err
		}
		if e == nil {
			return apperr.ErrNotFound
		}
		if e.ProjectID != nil {
			if err := s.adoptionRepository.Insert(ctx, id, externalID, *e.ProjectID); err != nil {
				return err
			}
		}
		return s.connectionRepository.ScheduleNow(ctx, id, forgetypes.TriggerManual)
	}))
}

func (s *UnlinkService) adopted(ctx context.Context, connID uuid.UUID, externalID string, projectID uuid.UUID) error {
	if err := s.sources.DropOwn(ctx, projectID); err != nil {
		return err
	}
	return s.adoptionRepository.Delete(ctx, connID, externalID)
}

type UnlinkService struct {
	adoptionRepository   *repositories.AdoptionRepository
	catalog              *catalogservices.CatalogServices
	connectionRepository *repositories.ConnectionRepository
	db                   *pgtx.DB
	exclusionRepository  *repositories.ExclusionRepository
	repoRepository       *repositories.RepoRepository
	sources              forgeports.ProjectSourcesPort
	connectionsService   *ConnectionsService
}
