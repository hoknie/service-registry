package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/forge"
	"svc-registry/internal/platform/apperr"
)

type ExclusionView struct {
	Exclusion forge.Exclusion
	Project   *ExcludedProject
}

type ExcludedProject struct {
	ID   uuid.UUID
	Name string
	Path string
}

func (s *Service) ProjectLink(ctx context.Context, id uuid.UUID) (*forge.ProjectLink, error) {
	l, err := s.repositories.Link(ctx, id)
	return l, apperr.Wrap(err)
}

func (s *Service) UnlinkProject(ctx context.Context, p access.Principal, id uuid.UUID) error {
	if err := s.catalog.AuthorizeProject(ctx, p, catalog.PermWrite, id); err != nil {
		return err
	}
	link, err := s.repositories.Link(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if link == nil {
		return apperr.Wrap(forge.ConflictNotLinked)
	}
	if _, err := s.catalog.Authorize(ctx, p, catalog.PermWrite, link.NodeID); err != nil {
		return err
	}
	return apperr.Wrap(s.db.InTx(ctx, func(ctx context.Context) error {
		u, err := s.repositories.Unlink(ctx, id)
		if err != nil {
			return err
		}
		if u == nil {
			return forge.ConflictNotLinked
		}
		if err := s.exclusions.Insert(ctx, *u); err != nil {
			return err
		}
		src := forge.RemoteSource{Kind: link.Kind, WebURL: link.WebURL, APIURL: link.APIURL}
		if link.Credentials.Kind == catalog.CredentialsSecret {
			src.SecretID = link.Credentials.SecretID
		}
		return s.sources.AdoptRemote(ctx, id, src)
	}))
}

func (s *Service) ListExclusions(ctx context.Context, p access.Principal, nodeID, id uuid.UUID) ([]ExclusionView, error) {
	if _, err := s.authorizeConnection(ctx, p, catalog.PermRead, nodeID, id); err != nil {
		return nil, err
	}
	list, err := s.exclusions.List(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make([]ExclusionView, 0, len(list))
	for _, e := range list {
		v := ExclusionView{Exclusion: e}
		if e.ProjectID != nil {
			if sc, err := s.catalog.Authorize(ctx, p, catalog.PermRead, *e.ProjectID); err == nil {
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

func (s *Service) DeleteExclusion(ctx context.Context, p access.Principal, nodeID, id uuid.UUID, externalID string) error {
	if _, err := s.authorizeConnection(ctx, p, catalog.PermAccess, nodeID, id); err != nil {
		return err
	}
	return apperr.Wrap(s.db.InTx(ctx, func(ctx context.Context) error {
		e, err := s.exclusions.Delete(ctx, id, externalID)
		if err != nil {
			return err
		}
		if e == nil {
			return forge.ErrNotFound
		}
		if e.ProjectID != nil {
			if err := s.adoptions.Insert(ctx, id, externalID, *e.ProjectID); err != nil {
				return err
			}
		}
		return s.connections.ScheduleNow(ctx, id, forge.TriggerManual)
	}))
}

func (s *Service) adopted(ctx context.Context, connID uuid.UUID, externalID string, projectID uuid.UUID) error {
	if err := s.sources.DropOwn(ctx, projectID); err != nil {
		return err
	}
	return s.adoptions.Delete(ctx, connID, externalID)
}
