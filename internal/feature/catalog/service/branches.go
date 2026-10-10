package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) AuthorizeProject(ctx context.Context, p access.Principal, perm catalog.Permission, id uuid.UUID) error {
	sc, err := s.Authorize(ctx, p, perm, id)
	if err != nil {
		return err
	}
	if sc.Node.Kind != catalog.KindProject {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func (s *Service) ListBranches(ctx context.Context, p access.Principal, id uuid.UUID, q catalog.BranchQuery) ([]catalog.Branch, catalog.BranchFilter, uint64, error) {
	if err := s.AuthorizeProject(ctx, p, catalog.PermRead, id); err != nil {
		return nil, catalog.BranchFilter{}, 0, err
	}
	f, err := catalog.ValidateBranchQuery(q)
	if err != nil {
		return nil, catalog.BranchFilter{}, 0, apperr.Wrap(err)
	}
	items, total, err := s.branches.List(ctx, id, f)
	return items, f, total, apperr.Wrap(err)
}

func (s *Service) GetBranch(ctx context.Context, p access.Principal, id uuid.UUID, rawName string) (catalog.Branch, error) {
	if err := s.AuthorizeProject(ctx, p, catalog.PermRead, id); err != nil {
		return catalog.Branch{}, err
	}
	name, err := catalog.BranchName(rawName)
	if err != nil {
		return catalog.Branch{}, apperr.Wrap(err)
	}
	b, err := s.branches.Get(ctx, id, name)
	if err != nil {
		return catalog.Branch{}, apperr.Wrap(err)
	}
	if b == nil {
		return catalog.Branch{}, apperr.New(apperr.NotFound)
	}
	return *b, nil
}

func (s *Service) PinBranch(ctx context.Context, p access.Principal, id uuid.UUID, rawName string, pinned bool) (catalog.Branch, error) {
	if err := s.AuthorizeProject(ctx, p, catalog.PermWrite, id); err != nil {
		return catalog.Branch{}, err
	}
	name, err := catalog.BranchName(rawName)
	if err != nil {
		return catalog.Branch{}, apperr.Wrap(err)
	}
	b, err := s.branches.SetPinned(ctx, id, name, pinned)
	if err != nil {
		return catalog.Branch{}, apperr.Wrap(err)
	}
	if b == nil {
		return catalog.Branch{}, apperr.New(apperr.NotFound)
	}
	return *b, nil
}

func (s *Service) DeleteBranch(ctx context.Context, p access.Principal, id uuid.UUID, rawName string) error {
	if err := s.AuthorizeProject(ctx, p, catalog.PermWrite, id); err != nil {
		return err
	}
	name, err := catalog.BranchName(rawName)
	if err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(s.branches.Delete(ctx, id, name))
}

func (s *Service) PruneBranches(ctx context.Context) (int64, error) {
	cfg := s.cfg.Branches
	n, err := s.branches.Prune(ctx, cfg.RetentionDays, cfg.StaleDays)
	return n, apperr.Wrap(err)
}
