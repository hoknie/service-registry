package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

type TemplateRepository struct{ db *postgres.DB }

func NewTemplateRepository(db *postgres.DB) *TemplateRepository { return &TemplateRepository{db: db} }

func lockNode(ctx context.Context, tx postgres.Querier, nodeID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		SELECT 1
		FROM nodes
		WHERE id = $1 FOR NO KEY UPDATE`, nodeID)
	return err
}

func (s *TemplateRepository) Effective(ctx context.Context, nodeID uuid.UUID) ([]linkstypes.Template, error) {
	return effectiveTemplates(ctx, s.db.From(ctx), nodeID)
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func effectiveTemplates(ctx context.Context, q querier, nodeID uuid.UUID) ([]linkstypes.Template, error) {
	rows, err := q.Query(ctx, `
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
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[templateRow])
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out := make([]linkstypes.Template, 0, len(items))
	for _, r := range items {
		out = append(out, r.template())
	}
	linkstypes.SortTemplates(out)
	return out, nil
}

func effectiveVars(ctx context.Context, q querier, nodeID uuid.UUID) ([]linkstypes.Var, error) {
	rows, err := q.Query(ctx, `
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
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[varRow])
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out := make([]linkstypes.Var, 0, len(items))
	for _, r := range items {
		out = append(out, linkstypes.Var(r))
	}
	return out, nil
}

func (s *TemplateRepository) Put(ctx context.Context, t linkstypes.NewTemplate) (linkstypes.Template, error) {
	var out linkstypes.Template
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		if err := lockNode(ctx, tx, t.NodeID); err != nil {
			return err
		}
		var kindID uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT id
			FROM link_kinds
			WHERE key = $1`, t.KindKey).Scan(&kindID); errors.Is(err, pgx.ErrNoRows) {
			return linkstypes.UnknownKind
		} else if err != nil {
			return err
		}
		var others int64
		if err := tx.QueryRow(ctx, `
			SELECT count(*)
			FROM link_templates
			WHERE node_id = $1
				AND link_key <> $2`, t.NodeID, t.LinkKey).Scan(&others); err != nil {
			return err
		}
		if others >= linkstypes.MaxTemplatesPerNode {
			return linkstypes.TooManyTemplates
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO link_templates (id, node_id, kind_id, link_key, template, disabled, position, title, icon_url, icon_file)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (node_id, link_key)
			DO UPDATE SET kind_id = EXCLUDED.kind_id, template = EXCLUDED.template,
				disabled = EXCLUDED.disabled, position = EXCLUDED.position, title = EXCLUDED.title,
				icon_url = EXCLUDED.icon_url, icon_file = EXCLUDED.icon_file, updated_at = now()`,
			t.ID, t.NodeID, kindID, t.LinkKey, t.Template, t.Disabled, t.Position, t.Title, t.IconURL, t.IconFile); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT t.id, t.node_id, t.link_key, k.key AS kind_key, k.position AS kind_position, t.template,
				t.disabled, t.position, t.title, t.icon_url, t.icon_file, k.icon AS kind_icon, k.icon_url AS kind_icon_url, rfc3339(t.created_at) AS created_at, rfc3339(t.updated_at) AS updated_at,
				false AS inherited
			FROM link_templates t
			JOIN link_kinds k ON k.id = t.kind_id
			WHERE t.node_id = $1
				AND t.link_key = $2`, t.NodeID, t.LinkKey)
		if err != nil {
			return err
		}
		r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[templateRow])
		out = r.template()
		return err
	})
	return out, postgres.Translate(err, constraints)
}

func (s *TemplateRepository) Delete(ctx context.Context, nodeID uuid.UUID, linkKey string) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM link_templates
		WHERE node_id = $1
			AND link_key = $2`, nodeID, linkKey)
	if err != nil {
		return postgres.Translate(err, constraints)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func (s *TemplateRepository) EffectiveVars(ctx context.Context, nodeID uuid.UUID) ([]linkstypes.Var, error) {
	return effectiveVars(ctx, s.db.From(ctx), nodeID)
}

func (s *TemplateRepository) ReplaceVars(ctx context.Context, nodeID uuid.UUID, vars map[string]string) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		if err := lockNode(ctx, tx, nodeID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM node_vars
			WHERE node_id = $1`, nodeID); err != nil {
			return err
		}
		for k, v := range vars {
			if _, err := tx.Exec(ctx, `
				INSERT INTO node_vars (id, node_id, key, value)
				VALUES ($1, $2, $3, $4)`, uuid.Must(uuid.NewV7()), nodeID, k, v); err != nil {
				return err
			}
		}
		return nil
	})
	return postgres.Translate(err, constraints)
}

func (s *TemplateRepository) ProjectData(ctx context.Context, projectID uuid.UUID) (linkstypes.ProjectData, error) {
	var out linkstypes.ProjectData
	err := s.db.InTxOpts(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var err error
		if out.Templates, err = effectiveTemplates(ctx, tx, projectID); err != nil {
			return err
		}
		if out.Vars, err = effectiveVars(ctx, tx, projectID); err != nil {
			return err
		}
		var full string
		switch err := tx.QueryRow(ctx, `
			SELECT full_path
			FROM forge_repositories
			WHERE project_id = $1
				AND orphaned_at IS NULL`, projectID).Scan(&full); {
		case err == nil:
			out.RepoFullPath = &full
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT d.service, d.environment, d.version, d.commit_sha, d.cluster, d.namespace, d.url, d.links
			FROM service_environments e
			JOIN service_deployments d ON d.id = e.deployment_id
			WHERE e.project_id = $1`, projectID)
		if err != nil {
			return err
		}
		deployments, err := pgx.CollectRows(rows, pgx.RowToStructByName[deploymentRow])
		if err != nil {
			return err
		}
		for _, d := range deployments {
			out.Deployments = append(out.Deployments, linkstypes.Deployment(d))
		}
		out.Kinds, err = kindInfos(ctx, tx)
		return err
	})
	return out, postgres.Translate(err, constraints)
}

func (s *TemplateRepository) IsUnder(ctx context.Context, node, root uuid.UUID) (bool, error) {
	var ok bool
	err := s.db.From(ctx).QueryRow(ctx, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, node, root).Scan(&ok)
	return ok, postgres.Translate(err, constraints)
}

func (s *TemplateRepository) Projects(ctx context.Context, after uuid.UUID, limit int) ([]uuid.UUID, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT id
		FROM nodes
		WHERE kind = 'project'
			AND id > $1
		ORDER BY id
		LIMIT $2`, after, limit)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return ids, postgres.Translate(err, constraints)
}

func kindInfos(ctx context.Context, q querier) (map[string]linkstypes.KindInfo, error) {
	rows, err := q.Query(ctx, `
		SELECT key, position, icon, icon_url
		FROM link_kinds`)
	if err != nil {
		return nil, err
	}
	out := map[string]linkstypes.KindInfo{}
	var key string
	var info linkstypes.KindInfo
	_, err = pgx.ForEachRow(rows, []any{&key, &info.Position, &info.Icon, &info.IconURL}, func() error {
		out[key] = info
		return nil
	})
	return out, err
}
