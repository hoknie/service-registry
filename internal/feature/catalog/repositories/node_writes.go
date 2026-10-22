package repositories

import (
	"context"
	"errors"

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
	s := string(*f)
	return &s
}

func (s *NodeRepository) Insert(ctx context.Context, n catalogtypes.NewNode) (catalogtypes.Node, error) {
	rows, _ := s.db.From(ctx).Query(ctx, `
		INSERT INTO nodes AS n (id, kind, parent_id, slug, name, description, labels, forge, repo_url,
			default_branch, cluster_observation)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, COALESCE($11, true))
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, n.ID, string(n.Kind), n.ParentID, n.Slug, n.Name,
		n.Description, labelsParam(n.Labels), forgeText(n.Repo.Forge), n.Repo.RepoURL, n.Repo.DefaultBranch, n.ClusterObservation)
	node, err := oneNode(rows)
	return node, postgres.Translate(err, constraints)
}

func (s *NodeRepository) Update(ctx context.Context, id uuid.UUID, c catalogtypes.NodeChanges) (catalogtypes.Node, error) {
	var labels *catalogtypes.Labels
	if c.Labels != nil {
		labels = &c.Labels
	}
	rows, _ := s.db.From(ctx).Query(ctx, `
		UPDATE nodes AS n
		SET slug = COALESCE($2, n.slug), name = COALESCE($3, n.name),
			description = COALESCE($4, n.description), labels = COALESCE($5::jsonb, n.labels),
			forge = CASE WHEN $6 THEN $7 ELSE n.forge END, repo_url = CASE WHEN $8 THEN $9 ELSE n.repo_url END,
			default_branch = CASE WHEN $10 THEN $11 ELSE n.default_branch END,
			cluster_observation = COALESCE($12, n.cluster_observation), updated_at = now()
		WHERE n.id = $1
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, id, c.Slug, c.Name, c.Description, labels,
		c.Forge.Set, forgeText(c.Forge.Value), c.RepoURL.Set, c.RepoURL.Value,
		c.DefaultBranch.Set, c.DefaultBranch.Value, c.ClusterObservation)
	node, err := oneNode(rows)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalogtypes.Node{}, apperr.ErrNotFound
	}
	return node, postgres.Translate(err, constraints)
}

func (s *NodeRepository) Move(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (catalogtypes.Node, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return catalogtypes.Node{}, postgres.Translate(err, constraints)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(7316001)`); err != nil {
		return catalogtypes.Node{}, postgres.Translate(err, constraints)
	}
	if parent != nil {
		var cycle bool
		if err := tx.QueryRow(ctx, `
			WITH RECURSIVE up AS (
				SELECT id, parent_id
				FROM nodes
				WHERE id = $1
				UNION ALL
				SELECT p.id, p.parent_id
				FROM nodes p
				JOIN up ON p.id = up.parent_id
			)
			SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, *parent, id).Scan(&cycle); err != nil {
			return catalogtypes.Node{}, postgres.Translate(err, constraints)
		}
		if cycle {
			return catalogtypes.Node{}, catalogtypes.ConflictCycle
		}
	}
	rows, _ := tx.Query(ctx, `
		UPDATE nodes AS n
		SET parent_id = $2, updated_at = now()
		WHERE n.id = $1
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, id, parent)
	node, err := oneNode(rows)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalogtypes.Node{}, apperr.ErrNotFound
	}
	if err != nil {
		return catalogtypes.Node{}, postgres.Translate(err, constraints)
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogtypes.Node{}, postgres.Translate(err, constraints)
	}
	return node, nil
}

func (s *NodeRepository) FillDefaultBranch(ctx context.Context, id uuid.UUID, name string) (bool, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		UPDATE nodes
		SET default_branch = $2, updated_at = now()
		WHERE id = $1
			AND kind = 'project'
			AND default_branch IS NULL`, id, name)
	if err != nil {
		return false, postgres.Translate(err, constraints)
	}
	return tag.RowsAffected() == 1, nil
}

func (s *NodeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM nodes
		WHERE id = $1`, id)
	if err != nil {
		if postgres.ConstraintOf(err) == "nodes_parent_id_fkey" {
			return catalogtypes.ConflictNodeNotEmpty
		}
		return postgres.Translate(err, constraints)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}
