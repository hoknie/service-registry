package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/catalog/repository"
)

func (s *Service) FindNode(ctx context.Context, id uuid.UUID) (*catalog.Node, error) {
	return s.nodes.Get(ctx, id)
}

func (s *Service) FindChild(ctx context.Context, parent *uuid.UUID, slug string) (*catalog.Node, error) {
	return s.nodes.ChildBySlug(ctx, parent, slug)
}

func (s *Service) NodeChain(ctx context.Context, id uuid.UUID) (*catalog.NodeChain, error) {
	return s.nodes.Chain(ctx, uuid.Nil, id)
}

func (s *Service) InsertNode(ctx context.Context, n catalog.NewNode) (catalog.Node, error) {
	return s.nodes.Insert(ctx, n)
}

func (s *Service) ChangeNode(ctx context.Context, id uuid.UUID, c catalog.NodeChanges) (catalog.Node, error) {
	return s.nodes.Update(ctx, id, c)
}

func (s *Service) PlaceNode(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (catalog.Node, error) {
	return s.nodes.Move(ctx, id, parent)
}

func (s *Service) RemoveNode(ctx context.Context, id uuid.UUID) error { return s.nodes.Delete(ctx, id) }

func (s *Service) SetDefaultBranch(ctx context.Context, projectID uuid.UUID, name *string) error {
	if name != nil && !catalog.BranchFits(*name) {
		name = nil
	}
	return s.branches.SetDefault(ctx, projectID, name)
}

func (s *Service) SetDefaultBranchIfEmpty(ctx context.Context, projectID uuid.UUID, name string) error {
	if !catalog.BranchFits(name) {
		return nil
	}
	return s.db.InTx(ctx, func(ctx context.Context) error {
		set, err := s.nodes.FillDefaultBranch(ctx, projectID, name)
		if err != nil || !set {
			return err
		}
		return s.branches.SetDefault(ctx, projectID, &name)
	})
}

func (s *Service) SyncForgeBranches(ctx context.Context, projectID uuid.UUID, branches []catalog.ForgeBranch, complete bool) error {
	kept := make([]catalog.ForgeBranch, 0, len(branches))
	for _, b := range branches {
		if catalog.BranchFits(b.Name) {
			kept = append(kept, b)
		}
	}
	return s.branches.SyncForge(ctx, projectID, kept, complete)
}

func (s *Service) RecordIngestBranch(ctx context.Context, projectID uuid.UUID, name string, commit *string, at string) error {
	return repository.UpsertIngestBranch(ctx, s.db.From(ctx), projectID, name, commit, at)
}

func (s *Service) RecordClusterBranch(ctx context.Context, projectID uuid.UUID, name string, commit *string, activity *time.Time) error {
	if !catalog.BranchFits(name) {
		return nil
	}
	return repository.UpsertClusterBranch(ctx, s.db.From(ctx), projectID, name, commit, activity)
}

func (s *Service) SyncRepositoryBranches(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string) error {
	kept := make(map[string]string, len(heads))
	for name, head := range heads {
		if catalog.BranchFits(name) {
			kept[name] = head
		}
	}
	heads = kept
	return repository.SyncRepositoryBranches(ctx, s.db.From(ctx), projectID, heads, defaultBranch)
}
