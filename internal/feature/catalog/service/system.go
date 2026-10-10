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
	return s.branches.SetDefault(ctx, projectID, name)
}

func (s *Service) SyncForgeBranches(ctx context.Context, projectID uuid.UUID, branches []catalog.ForgeBranch, complete bool) error {
	return s.branches.SyncForge(ctx, projectID, branches, complete)
}

func (s *Service) RecordIngestBranch(ctx context.Context, projectID uuid.UUID, name string, commit *string, at string) error {
	return repository.UpsertIngestBranch(ctx, s.db.From(ctx), projectID, name, commit, at)
}

func (s *Service) RecordClusterBranch(ctx context.Context, projectID uuid.UUID, name string, commit *string, activity *time.Time) error {
	return repository.UpsertClusterBranch(ctx, s.db.From(ctx), projectID, name, commit, activity)
}

func (s *Service) SyncRepositoryBranches(ctx context.Context, projectID uuid.UUID, heads map[string]string, defaultBranch string) error {
	return repository.SyncRepositoryBranches(ctx, s.db.From(ctx), projectID, heads, defaultBranch)
}
