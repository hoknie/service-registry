package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

func hostingText(f *catalogtypes.Hosting) *string {
	if f == nil {
		return nil
	}
	return new(string(*f))
}

func (s *NodeRepository) Insert(ctx context.Context, n catalogtypes.NewNode) (catalogtypes.Node, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, scanNode, `
		INSERT INTO nodes AS n (id, kind, parent_id, slug, name, description, labels, hosting, repo_url,
			default_branch, cluster_observation)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11)
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.hosting, n.repo_url,
			n.default_branch, n.cluster_observation, n.visibility, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, n.ID, string(n.Kind), n.ParentID, n.Slug, n.Name,
		n.Description, n.Labels, hostingText(n.Repo.Hosting), n.Repo.RepoURL, n.Repo.DefaultBranch, n.ClusterObservation)
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
			hosting = CASE WHEN $6 THEN $7 ELSE n.hosting END, repo_url = CASE WHEN $8 THEN $9 ELSE n.repo_url END,
			default_branch = CASE WHEN $10 THEN $11 ELSE n.default_branch END,
			cluster_observation = COALESCE($12, n.cluster_observation), updated_at = now()
		WHERE n.id = $1
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.hosting, n.repo_url,
			n.default_branch, n.cluster_observation, n.visibility, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, id, c.Slug, c.Name, c.Description, labels,
		c.Hosting.Set, hostingText(c.Hosting.Value), c.RepoURL.Set, c.RepoURL.Value,
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

func (s *NodeRepository) LockTree(ctx context.Context) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		SELECT pg_advisory_xact_lock(7316001)`)
	return err
}

func (s *NodeRepository) IsAncestorOrSelf(ctx context.Context, ancestor, id uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		WITH RECURSIVE up AS (
			SELECT id, parent_id
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT p.id, p.parent_id
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, id, ancestor)
}

func (s *NodeRepository) SetParent(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (catalogtypes.Node, error) {
	return foundNode(postgres.Find(ctx, s.db.From(ctx), constraints, scanNode, `
		UPDATE nodes AS n
		SET parent_id = $2, visibility = CASE WHEN $2::uuid IS NULL THEN n.visibility ELSE 'closed' END, updated_at = now()
		WHERE n.id = $1
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.hosting, n.repo_url,
			n.default_branch, n.cluster_observation, n.visibility, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, id, parent))
}

func (s *NodeRepository) SetVisibility(ctx context.Context, id uuid.UUID, v catalogtypes.OrgVisibility) (catalogtypes.Node, error) {
	return foundNode(postgres.Find(ctx, s.db.From(ctx), constraints, scanNode, `
		UPDATE nodes AS n
		SET visibility = $2, updated_at = now()
		WHERE n.id = $1
			AND n.parent_id IS NULL
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.hosting, n.repo_url,
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
