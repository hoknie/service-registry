package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/links"
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

type batchDeploymentRow struct {
	ProjectID uuid.UUID `db:"project_id"`
	deploymentRow
}

func (s *Templates) ProjectsData(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]links.ProjectBatch, error) {
	out := make(map[uuid.UUID]links.ProjectBatch, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	err := s.db.InTxOpts(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		rows, err := tx.Query(ctx, `
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
		path, err := pgx.CollectRows(rows, pgx.RowToStructByName[batchPathRow])
		if err != nil {
			return err
		}
		for _, r := range path {
			b := out[r.ProjectID]
			b.Path = append(b.Path, links.PathNode{Slug: r.Slug, Labels: r.Labels})
			b.Name, b.DefaultBranch = r.Name, r.DefaultBranch
			out[r.ProjectID] = b
		}
		rows, err = tx.Query(ctx, `
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
				k.icon AS kind_icon, rfc3339(t.created_at) AS created_at, rfc3339(t.updated_at) AS updated_at,
				up.lvl > 0 AS inherited
			FROM up
			JOIN link_templates t ON t.node_id = up.id
			JOIN link_kinds k ON k.id = t.kind_id
			ORDER BY up.project_id, t.link_key, up.lvl`, ids)
		if err != nil {
			return err
		}
		templates, err := pgx.CollectRows(rows, pgx.RowToStructByName[batchTemplateRow])
		if err != nil {
			return err
		}
		for _, r := range templates {
			b := out[r.ProjectID]
			b.Templates = append(b.Templates, r.template())
			out[r.ProjectID] = b
		}
		rows, err = tx.Query(ctx, `
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
		vars, err := pgx.CollectRows(rows, pgx.RowToStructByName[batchVarRow])
		if err != nil {
			return err
		}
		for _, r := range vars {
			b := out[r.ProjectID]
			b.Vars = append(b.Vars, links.Var{Key: r.Key, Value: r.Value, NodeID: r.NodeID, Inherited: r.Inherited})
			out[r.ProjectID] = b
		}
		rows, err = tx.Query(ctx, `
			SELECT project_id, full_path
			FROM forge_repositories
			WHERE project_id = ANY($1)
				AND orphaned_at IS NULL`, ids)
		if err != nil {
			return err
		}
		var project uuid.UUID
		var full string
		if _, err := pgx.ForEachRow(rows, []any{&project, &full}, func() error {
			b := out[project]
			path := full
			b.RepoFullPath = &path
			out[project] = b
			return nil
		}); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, `
			SELECT e.project_id, d.service, d.environment, d.version, d.commit_sha, d.cluster, d.namespace, d.url
			FROM service_environments e
			JOIN service_deployments d ON d.id = e.deployment_id
			WHERE e.project_id = ANY($1)`, ids)
		if err != nil {
			return err
		}
		deployments, err := pgx.CollectRows(rows, pgx.RowToStructByName[batchDeploymentRow])
		if err != nil {
			return err
		}
		for _, d := range deployments {
			b := out[d.ProjectID]
			b.Deployments = append(b.Deployments, links.Deployment{Service: d.Service, Environment: d.Environment,
				Version: d.Version, Commit: d.Commit, Cluster: d.Cluster, Namespace: d.Namespace, URL: d.URL})
			out[d.ProjectID] = b
		}
		return nil
	})
	for id, b := range out {
		links.SortTemplates(b.Templates)
		out[id] = b
	}
	return out, dbErr(err)
}
