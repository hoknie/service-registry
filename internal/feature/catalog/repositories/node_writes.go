package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

func forgeText(f *catalogtypes.Forge) *string {
	if f == nil {
		return nil
	}
	return new(string(*f))
}

func (s *NodeRepository) Insert(ctx context.Context, n catalogtypes.NewNode) (catalogtypes.Node, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, scanNode, `
		INSERT INTO nodes AS n (id, kind, parent_id, slug, name, description, labels, forge, repo_url,
			default_branch, cluster_observation)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, COALESCE($11, true))
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, n.visibility, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, n.ID, string(n.Kind), n.ParentID, n.Slug, n.Name,
		n.Description, labelsParam(n.Labels), forgeText(n.Repo.Forge), n.Repo.RepoURL, n.Repo.DefaultBranch, n.ClusterObservation)
}

func (s *NodeRepository) Update(ctx context.Context, id uuid.UUID, c catalogtypes.NodeChanges) (catalogtypes.Node, error) {
	var labels *catalogtypes.Labels
	if c.Labels != nil {
		labels = &c.Labels
	}
	node, err := postgres.Find(ctx, s.db.From(ctx), constraints, scanNode, `
		UPDATE nodes AS n
		SET slug = COALESCE($2, n.slug), name = COALESCE($3, n.name),
			description = COALESCE($4, n.description), labels = COALESCE($5::jsonb, n.labels),
			forge = CASE WHEN $6 THEN $7 ELSE n.forge END, repo_url = CASE WHEN $8 THEN $9 ELSE n.repo_url END,
			default_branch = CASE WHEN $10 THEN $11 ELSE n.default_branch END,
			cluster_observation = COALESCE($12, n.cluster_observation), updated_at = now()
		WHERE n.id = $1
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, n.visibility, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, id, c.Slug, c.Name, c.Description, labels,
		c.Forge.Set, forgeText(c.Forge.Value), c.RepoURL.Set, c.RepoURL.Value,
		c.DefaultBranch.Set, c.DefaultBranch.Value, c.ClusterObservation)
	return foundNode(node, err)
}

func foundNode(node *catalogtypes.Node, err error) (catalogtypes.Node, error) {
	if err != nil {
		return catalogtypes.Node{}, err
	}
	if node == nil {
		return catalogtypes.Node{}, apperr.ErrNotFound
	}
	return *node, nil
}

func (s *NodeRepository) Move(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (catalogtypes.Node, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return catalogtypes.Node{}, postgres.Translate(err, constraints)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := postgres.Exec(ctx, tx, constraints, `
		SELECT pg_advisory_xact_lock(7316001)`); err != nil {
		return catalogtypes.Node{}, err
	}
	if parent != nil {
		cycle, err := postgres.One(ctx, tx, constraints, pgx.RowTo[bool], `
			WITH RECURSIVE up AS (
				SELECT id, parent_id
				FROM nodes
				WHERE id = $1
				UNION ALL
				SELECT p.id, p.parent_id
				FROM nodes p
				JOIN up ON p.id = up.parent_id
			)
			SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, *parent, id)
		if err != nil {
			return catalogtypes.Node{}, err
		}
		if cycle {
			return catalogtypes.Node{}, catalogtypes.ConflictCycle
		}
	}
	moved, err := foundNode(postgres.Find(ctx, tx, constraints, scanNode, `
		UPDATE nodes AS n
		SET parent_id = $2, visibility = CASE WHEN $2::uuid IS NULL THEN n.visibility ELSE 'closed' END, updated_at = now()
		WHERE n.id = $1
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, n.visibility, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, id, parent))
	if err != nil {
		return catalogtypes.Node{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogtypes.Node{}, postgres.Translate(err, constraints)
	}
	return moved, nil
}

func (s *NodeRepository) SetVisibility(ctx context.Context, id uuid.UUID, v catalogtypes.OrgVisibility) (catalogtypes.Node, error) {
	return foundNode(postgres.Find(ctx, s.db.From(ctx), constraints, scanNode, `
		UPDATE nodes AS n
		SET visibility = $2, updated_at = now()
		WHERE n.id = $1
			AND n.parent_id IS NULL
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, n.visibility, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, id, string(v)))
}

func (s *NodeRepository) FillDefaultBranch(ctx context.Context, id uuid.UUID, name string) (bool, error) {
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE nodes
		SET default_branch = $2, updated_at = now()
		WHERE id = $1
			AND kind = 'project'
			AND default_branch IS NULL`, id, name)
	return n == 1, err
}

func (s *NodeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), nodeDeleteConstraints, `
		DELETE FROM nodes
		WHERE id = $1`, id)
}
