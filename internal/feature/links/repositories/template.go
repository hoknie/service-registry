package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type TemplateRepository struct{ db *pgtx.DB }

func NewTemplateRepository(db *pgtx.DB) *TemplateRepository { return &TemplateRepository{db: db} }

func (s *TemplateRepository) LockNode(ctx context.Context, nodeID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		SELECT 1
		FROM nodes
		WHERE id = $1 FOR NO KEY UPDATE`, nodeID)
	return err
}

func (s *TemplateRepository) Effective(ctx context.Context, nodeID uuid.UUID) ([]linkstypes.Template, error) {
	return effectiveTemplates(ctx, s.db.From(ctx), nodeID)
}

func effectiveTemplates(ctx context.Context, q pgtx.Querier, nodeID uuid.UUID) ([]linkstypes.Template, error) {
	return postgres.List(ctx, q, constraints, postgres.Struct(templateRow.template), `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT DISTINCT ON (t.link_key) t.id, t.node_id, t.link_key, k.key AS kind_key,
			k.position AS kind_position, t.template, t.disabled, t.position, t.title, t.icon_url, t.icon_file,
			k.icon AS kind_icon, k.icon_url AS kind_icon_url, rfc3339(t.created_at) AS created_at, rfc3339(t.updated_at) AS updated_at, up.lvl > 0 AS inherited
		FROM up
		JOIN link_templates t ON t.node_id = up.id
		JOIN link_kinds k ON k.id = t.kind_id
		ORDER BY t.link_key, up.lvl`, nodeID)
}

func effectiveVars(ctx context.Context, q pgtx.Querier, nodeID uuid.UUID) ([]linkstypes.Var, error) {
	return postgres.List(ctx, q, constraints, postgres.Struct(varRow.variable), `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT DISTINCT ON (v.key) v.key, v.value, v.node_id, up.lvl > 0 AS inherited
		FROM up
		JOIN node_vars v ON v.node_id = up.id
		ORDER BY v.key, up.lvl`, nodeID)
}

func (s *TemplateRepository) KindID(ctx context.Context, key string) (*uuid.UUID, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT id
		FROM link_kinds
		WHERE key = $1`, key)
}

func (s *TemplateRepository) CountOthers(ctx context.Context, nodeID uuid.UUID, linkKey string) (int64, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM link_templates
		WHERE node_id = $1
			AND link_key <> $2`, nodeID, linkKey)
}

func (s *TemplateRepository) Upsert(ctx context.Context, t linkstypes.NewTemplate, kindID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO link_templates (id, node_id, kind_id, link_key, template, disabled, position, title, icon_url, icon_file)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (node_id, link_key)
		DO UPDATE SET kind_id = EXCLUDED.kind_id, template = EXCLUDED.template,
			disabled = EXCLUDED.disabled, position = EXCLUDED.position, title = EXCLUDED.title,
			icon_url = EXCLUDED.icon_url, icon_file = EXCLUDED.icon_file, updated_at = now()`,
		t.ID, t.NodeID, kindID, t.LinkKey, t.Template, t.Disabled, t.Position, t.Title, t.IconURL, t.IconFile)
	return err
}

func (s *TemplateRepository) Own(ctx context.Context, nodeID uuid.UUID, linkKey string) (linkstypes.Template, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, postgres.Struct(templateRow.template), `
		SELECT t.id, t.node_id, t.link_key, k.key AS kind_key, k.position AS kind_position, t.template,
			t.disabled, t.position, t.title, t.icon_url, t.icon_file, k.icon AS kind_icon, k.icon_url AS kind_icon_url, rfc3339(t.created_at) AS created_at, rfc3339(t.updated_at) AS updated_at,
			false AS inherited
		FROM link_templates t
		JOIN link_kinds k ON k.id = t.kind_id
		WHERE t.node_id = $1
			AND t.link_key = $2`, nodeID, linkKey)
}

func (s *TemplateRepository) Delete(ctx context.Context, nodeID uuid.UUID, linkKey string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM link_templates
		WHERE node_id = $1
			AND link_key = $2`, nodeID, linkKey)
}

func (s *TemplateRepository) EffectiveVars(ctx context.Context, nodeID uuid.UUID) ([]linkstypes.Var, error) {
	return effectiveVars(ctx, s.db.From(ctx), nodeID)
}

func (s *TemplateRepository) DeleteVars(ctx context.Context, nodeID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM node_vars
		WHERE node_id = $1`, nodeID)
	return err
}

func (s *TemplateRepository) InsertVar(ctx context.Context, id, nodeID uuid.UUID, key, value string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO node_vars (id, node_id, key, value)
		VALUES ($1, $2, $3, $4)`, id, nodeID, key, value)
	return err
}

func (s *TemplateRepository) ProjectData(ctx context.Context, projectID uuid.UUID) (linkstypes.ProjectData, error) {
	var out linkstypes.ProjectData
	tx := s.db.From(ctx)
	var err error
	if out.Templates, err = effectiveTemplates(ctx, tx, projectID); err != nil {
		return out, err
	}
	if out.Vars, err = effectiveVars(ctx, tx, projectID); err != nil {
		return out, err
	}
	if out.RepoFullPath, err = postgres.Find(ctx, tx, constraints, pgx.RowTo[string], `
		SELECT full_path
		FROM repo_sync_repositories
		WHERE project_id = $1
			AND orphaned_at IS NULL`, projectID); err != nil {
		return out, err
	}
	deployments, err := postgres.List(ctx, tx, constraints, pgx.RowToStructByName[deploymentRow], `
		SELECT d.service, d.environment, d.version, d.commit_sha, d.cluster, d.namespace, d.url, d.links
		FROM service_environments e
		JOIN service_deployments d ON d.id = e.deployment_id
		WHERE e.project_id = $1`, projectID)
	if err != nil {
		return out, err
	}
	for _, d := range deployments {
		out.Deployments = append(out.Deployments, linkstypes.Deployment(d))
	}
	out.Kinds, err = kindInfos(ctx, tx)
	return out, err
}

func (s *TemplateRepository) IsUnder(ctx context.Context, node, root uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, node, root)
}

func (s *TemplateRepository) Projects(ctx context.Context, after uuid.UUID, limit int) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT id
		FROM nodes
		WHERE kind = 'project'
			AND id > $1
		ORDER BY id
		LIMIT $2`, after, limit)
}

func (s *TemplateRepository) ReferencedIcons(ctx context.Context, ids []string) ([]string, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		SELECT DISTINCT icon_file
		FROM link_templates
		WHERE icon_file = ANY($1)`, ids)
}

func kindInfos(ctx context.Context, q pgtx.Querier) (map[string]linkstypes.KindInfo, error) {
	items, err := postgres.List(ctx, q, constraints, pgx.RowToStructByName[kindInfoRow], `
		SELECT key, position, icon, icon_url
		FROM link_kinds`)
	if err != nil {
		return nil, err
	}
	out := make(map[string]linkstypes.KindInfo, len(items))
	for _, r := range items {
		out[r.Key] = linkstypes.KindInfo{Position: r.Position, Icon: r.Icon, IconURL: r.IconURL}
	}
	return out, nil
}
