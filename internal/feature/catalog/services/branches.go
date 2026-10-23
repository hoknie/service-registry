package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

func (s *BranchesService) AuthorizeProject(ctx context.Context, p accesstypes.Principal, perm catalogtypes.Permission, id uuid.UUID) error {
	sc, err := s.access.Authorize(ctx, p, perm, id)
	if err != nil {
		return err
	}
	if sc.Node.Kind != catalogtypes.KindProject {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func (s *BranchesService) ListBranches(ctx context.Context, p accesstypes.Principal, id uuid.UUID, q catalogtypes.BranchQuery) ([]catalogtypes.Branch, catalogtypes.BranchFilter, uint64, error) {
	if err := s.AuthorizeProject(ctx, p, catalogtypes.PermRead, id); err != nil {
		return nil, catalogtypes.BranchFilter{}, 0, err
	}
	f, err := catalogtypes.ValidateBranchQuery(q)
	if err != nil {
		return nil, catalogtypes.BranchFilter{}, 0, apperr.Wrap(err)
	}
	items, total, err := s.branchRepository.List(ctx, id, f)
	return items, f, total, apperr.Wrap(err)
}

func (s *BranchesService) GetBranch(ctx context.Context, p accesstypes.Principal, id uuid.UUID, rawName string) (catalogtypes.Branch, error) {
	if err := s.AuthorizeProject(ctx, p, catalogtypes.PermRead, id); err != nil {
		return catalogtypes.Branch{}, err
	}
	name, err := catalogtypes.BranchName(rawName)
	if err != nil {
		return catalogtypes.Branch{}, apperr.Wrap(err)
	}
	b, err := s.branchRepository.Get(ctx, id, name)
	if err != nil {
		return catalogtypes.Branch{}, apperr.Wrap(err)
	}
	if b == nil {
		return catalogtypes.Branch{}, apperr.New(apperr.NotFound)
	}
	return *b, nil
}

func (s *BranchesService) PinBranch(ctx context.Context, p accesstypes.Principal, id uuid.UUID, rawName string, pinned bool) (catalogtypes.Branch, error) {
	if err := s.AuthorizeProject(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return catalogtypes.Branch{}, err
	}
	name, err := catalogtypes.BranchName(rawName)
	if err != nil {
		return catalogtypes.Branch{}, apperr.Wrap(err)
	}
	b, err := s.branchRepository.SetPinned(ctx, id, name, pinned)
	if err != nil {
		return catalogtypes.Branch{}, apperr.Wrap(err)
	}
	if b == nil {
		return catalogtypes.Branch{}, apperr.New(apperr.NotFound)
	}
	return *b, nil
}

func (s *BranchesService) DeleteBranch(ctx context.Context, p accesstypes.Principal, id uuid.UUID, rawName string) error {
	if err := s.AuthorizeProject(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return err
	}
	name, err := catalogtypes.BranchName(rawName)
	if err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(s.branchRepository.Delete(ctx, id, name))
}

func (s *BranchesService) PruneBranches(ctx context.Context) (int64, error) {
	cfg := s.cfg.Branches
	n, err := s.branchRepository.Prune(ctx, cfg.RetentionDays, cfg.StaleDays)
	return n, apperr.Wrap(err)
}

type BranchesService struct {
	branchRepository *repositories.BranchRepository
	cfg              *config.Config
	access           *AccessService
}

func (s *BranchRecordsService) SetDefaultBranch(ctx context.Context, projectID uuid.UUID, name *string) error {
	if name != nil && !catalogtypes.BranchFits(*name) {
		name = nil
	}
	return s.branchRepository.SetDefault(ctx, projectID, name)
}

func (s *BranchRecordsService) SetDefaultBranchIfEmpty(ctx context.Context, projectID uuid.UUID, name string) error {
	if !catalogtypes.BranchFits(name) {
		return nil
	}
	return s.db.InTx(ctx, func(ctx context.Context) error {
		set, err := s.nodeRepository.FillDefaultBranch(ctx, projectID, name)
		if err != nil || !set {
			return err
		}
		return s.branchRepository.SetDefault(ctx, projectID, &name)
	})
}

func (s *BranchRecordsService) SyncForgeBranches(ctx context.Context, projectID uuid.UUID, branches []catalogtypes.ForgeBranch, complete bool) (bool, error) {
	kept := make([]catalogtypes.ForgeBranch, 0, len(branches))
	for _, b := range branches {
		if catalogtypes.BranchFits(b.Name) {
			kept = append(kept, b)
		}
	}
	return s.branchRepository.SyncForge(ctx, projectID, kept, complete)
}

func (s *BranchRecordsService) RecordIngestBranch(ctx context.Context, projectID uuid.UUID, name string, commit *string, at string) error {
	return repositories.UpsertIngestBranch(ctx, s.db.From(ctx), projectID, name, commit, at)
}

func (s *BranchRecordsService) RecordClusterBranch(ctx context.Context, projectID uuid.UUID, name string, commit *string, activity *time.Time) error {
	if !catalogtypes.BranchFits(name) {
		return nil
	}
	return repositories.UpsertClusterBranch(ctx, s.db.From(ctx), projectID, name, commit, activity)
}

func (s *BranchRecordsService) SyncRepositoryBranches(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string) error {
	kept := make(map[string]string, len(heads))
	for name, head := range heads {
		if catalogtypes.BranchFits(name) {
			kept[name] = head
		}
	}
	heads = kept
	return repositories.SyncRepositoryBranches(ctx, s.db.From(ctx), projectID, heads, defaultBranch)
}

type BranchRecordsService struct {
	branchRepository *repositories.BranchRepository
	db               *pgtx.DB
	nodeRepository   *repositories.NodeRepository
}
