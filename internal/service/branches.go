package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
)

func authorizeProject(ctx context.Context, state *State, p Principal, perm catalog.Permission, id uuid.UUID) error {
	s, err := Authorize(ctx, state, p, perm, id)
	if err != nil {
		return err
	}
	if s.Node.Kind != catalog.KindProject {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func ListBranches(ctx context.Context, state *State, p Principal, id uuid.UUID, q catalog.BranchQuery) ([]catalog.Branch, catalog.BranchFilter, uint64, error) {
	if err := authorizeProject(ctx, state, p, catalog.PermRead, id); err != nil {
		return nil, catalog.BranchFilter{}, 0, err
	}
	f, err := catalog.ValidateBranchQuery(q)
	if err != nil {
		return nil, catalog.BranchFilter{}, 0, apperr.Wrap(err)
	}
	items, total, err := state.Branches.List(ctx, id, f)
	return items, f, total, apperr.Wrap(err)
}

func GetBranch(ctx context.Context, state *State, p Principal, id uuid.UUID, rawName string) (catalog.Branch, error) {
	if err := authorizeProject(ctx, state, p, catalog.PermRead, id); err != nil {
		return catalog.Branch{}, err
	}
	name, err := catalog.BranchName(rawName)
	if err != nil {
		return catalog.Branch{}, apperr.Wrap(err)
	}
	b, err := state.Branches.Get(ctx, id, name)
	if err != nil {
		return catalog.Branch{}, apperr.Wrap(err)
	}
	if b == nil {
		return catalog.Branch{}, apperr.New(apperr.NotFound)
	}
	return *b, nil
}

func PinBranch(ctx context.Context, state *State, p Principal, id uuid.UUID, rawName string, pinned bool) (catalog.Branch, error) {
	if err := authorizeProject(ctx, state, p, catalog.PermWrite, id); err != nil {
		return catalog.Branch{}, err
	}
	name, err := catalog.BranchName(rawName)
	if err != nil {
		return catalog.Branch{}, apperr.Wrap(err)
	}
	b, err := state.Branches.SetPinned(ctx, id, name, pinned)
	if err != nil {
		return catalog.Branch{}, apperr.Wrap(err)
	}
	if b == nil {
		return catalog.Branch{}, apperr.New(apperr.NotFound)
	}
	return *b, nil
}

func DeleteBranch(ctx context.Context, state *State, p Principal, id uuid.UUID, rawName string) error {
	if err := authorizeProject(ctx, state, p, catalog.PermWrite, id); err != nil {
		return err
	}
	name, err := catalog.BranchName(rawName)
	if err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(state.Branches.Delete(ctx, id, name))
}

func PruneBranches(ctx context.Context, state *State) (int64, error) {
	cfg := state.Config.Branches
	n, err := state.Branches.Prune(ctx, cfg.RetentionDays, cfg.StaleDays)
	return n, apperr.Wrap(err)
}
