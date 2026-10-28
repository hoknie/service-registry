package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type batchPathRow struct {
	ProjectID     uuid.UUID         `db:"project_id"`
	Slug          string            `db:"slug"`
	Name          string            `db:"name"`
	Labels        map[string]string `db:"labels"`
	DefaultBranch *string           `db:"default_branch"`
}

type batchTemplateRow struct {
	ProjectID uuid.UUID `db:"project_id"`
	templateRow
}

type batchVarRow struct {
	ProjectID uuid.UUID `db:"project_id"`
	varRow
}

type batchRepoRow struct {
	ProjectID uuid.UUID `db:"project_id"`
	FullPath  string    `db:"full_path"`
}

type batchDeploymentRow struct {
	ProjectID uuid.UUID `db:"project_id"`
	deploymentRow
}

func (s *TemplateRepository) ProjectsData(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]linkstypes.ProjectBatch, error) {
	out := make(map[uuid.UUID]linkstypes.ProjectBatch, len(ids))
	tx := s.db.From(ctx)
	for _, load := range []func(context.Context, pgtx.Querier, []uuid.UUID, map[uuid.UUID]linkstypes.ProjectBatch) error{
		batchPaths, batchTemplates, batchVars, batchRepos, batchDeployments,
	} {
		if err := load(ctx, tx, ids, out); err != nil {
			return nil, err
		}
	}
	kinds, err := kindInfos(ctx, tx)
	if err != nil {
		return nil, err
	}
	for id, b := range out {
		b.Kinds = kinds
		out[id] = b
	}
	return out, nil
}

func batchPaths(ctx context.Context, tx pgtx.Querier, ids []uuid.UUID, out map[uuid.UUID]linkstypes.ProjectBatch) error {
	path, err := postgres.List(ctx, tx, constraints, pgx.RowToStructByName[batchPathRow], `
		WITH RECURSIVE up AS (
			SELECT id AS project_id, id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = ANY($1)
			UNION ALL
			SELECT up.project_id, p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
			WHERE up.lvl < 64
		)
		SELECT up.project_id, n.slug, n.name, n.labels, n.default_branch
		FROM up
		JOIN nodes n ON n.id = up.id
		ORDER BY up.project_id, up.lvl DESC`, ids)
	if err != nil {
		return err
	}
	for _, r := range path {
		b := out[r.ProjectID]
		b.Path = append(b.Path, linkstypes.PathNode{Slug: r.Slug, Labels: r.Labels})
		b.Name, b.DefaultBranch = r.Name, r.DefaultBranch
		out[r.ProjectID] = b
	}
	return nil
}

func batchTemplates(ctx context.Context, tx pgtx.Querier, ids []uuid.UUID, out map[uuid.UUID]linkstypes.ProjectBatch) error {
	templates, err := postgres.List(ctx, tx, constraints, pgx.RowToStructByName[batchTemplateRow], `
		WITH RECURSIVE up AS (
			SELECT id AS project_id, id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = ANY($1)
			UNION ALL
			SELECT up.project_id, p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
			WHERE up.lvl < 64
		)
		SELECT DISTINCT ON (up.project_id, t.link_key) up.project_id, t.id, t.node_id, t.link_key, k.key AS kind_key,
			k.position AS kind_position, t.template, t.disabled, t.position, t.title, t.icon_url, t.icon_file,
			k.icon AS kind_icon, k.icon_url AS kind_icon_url, rfc3339(t.created_at) AS created_at, rfc3339(t.updated_at) AS updated_at,
			up.lvl > 0 AS inherited
		FROM up
		JOIN link_templates t ON t.node_id = up.id
		JOIN link_kinds k ON k.id = t.kind_id
		ORDER BY up.project_id, t.link_key, up.lvl`, ids)
	if err != nil {
		return err
	}
	for _, r := range templates {
		b := out[r.ProjectID]
		b.Templates = append(b.Templates, r.template())
		out[r.ProjectID] = b
	}
	return nil
}

func batchVars(ctx context.Context, tx pgtx.Querier, ids []uuid.UUID, out map[uuid.UUID]linkstypes.ProjectBatch) error {
	vars, err := postgres.List(ctx, tx, constraints, pgx.RowToStructByName[batchVarRow], `
		WITH RECURSIVE up AS (
			SELECT id AS project_id, id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = ANY($1)
			UNION ALL
			SELECT up.project_id, p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
			WHERE up.lvl < 64
		)
		SELECT DISTINCT ON (up.project_id, v.key) up.project_id, v.key, v.value, v.node_id, up.lvl > 0 AS inherited
		FROM up
		JOIN node_vars v ON v.node_id = up.id
		ORDER BY up.project_id, v.key, up.lvl`, ids)
	if err != nil {
		return err
	}
	for _, r := range vars {
		b := out[r.ProjectID]
		b.Vars = append(b.Vars, linkstypes.Var{Key: r.Key, Value: r.Value, NodeID: r.NodeID, Inherited: r.Inherited})
		out[r.ProjectID] = b
	}
	return nil
}

func batchRepos(ctx context.Context, tx pgtx.Querier, ids []uuid.UUID, out map[uuid.UUID]linkstypes.ProjectBatch) error {
	repos, err := postgres.List(ctx, tx, constraints, pgx.RowToStructByName[batchRepoRow], `
		SELECT project_id, full_path
		FROM repo_sync_repositories
		WHERE project_id = ANY($1)
			AND orphaned_at IS NULL`, ids)
	if err != nil {
		return err
	}
	for _, r := range repos {
		b := out[r.ProjectID]
		b.RepoFullPath = new(r.FullPath)
		out[r.ProjectID] = b
	}
	return nil
}

func batchDeployments(ctx context.Context, tx pgtx.Querier, ids []uuid.UUID, out map[uuid.UUID]linkstypes.ProjectBatch) error {
	deployments, err := postgres.List(ctx, tx, constraints, pgx.RowToStructByName[batchDeploymentRow], `
		SELECT e.project_id, d.service, d.environment, d.version, d.commit_sha, d.cluster, d.namespace, d.url, d.links
		FROM service_environments e
		JOIN service_deployments d ON d.id = e.deployment_id
		WHERE e.project_id = ANY($1)`, ids)
	if err != nil {
		return err
	}
	for _, d := range deployments {
		b := out[d.ProjectID]
		b.Deployments = append(b.Deployments, linkstypes.Deployment(d.deploymentRow))
		out[d.ProjectID] = b
	}
	return nil
}
