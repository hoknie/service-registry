package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type FolderRepository struct{ db *pgtx.DB }

func NewFolderRepository(db *pgtx.DB) *FolderRepository { return &FolderRepository{db: db} }

func (s *FolderRepository) Chain(ctx context.Context, id uuid.UUID) ([]gatewaytypes.Folder, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(folderRow.folder), `
		WITH RECURSIVE up AS (
			SELECT f.*, 0 AS lvl
			FROM mcp_folders f
			WHERE f.id = $1
			UNION ALL
			SELECT p.*, up.lvl + 1
			FROM mcp_folders p
			JOIN up ON p.id = up.parent_id
		)
		SELECT up.id, up.parent_id, up.organization_id, o.slug AS org_slug, o.name AS org_name, up.slug, up.name,
			up.description, up.lvl AS depth, 0::bigint AS servers, 0::bigint AS folders,
			rfc3339(up.created_at) AS created_at, rfc3339(up.updated_at) AS updated_at
		FROM up
		LEFT JOIN nodes o ON o.id = up.organization_id
		ORDER BY up.lvl`, id)
}

func (s *FolderRepository) List(ctx context.Context, parent *uuid.UUID, v, w gatewaytypes.Visible) ([]gatewaytypes.Folder, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(folderRow.folder), `
		SELECT f.id, f.parent_id, f.organization_id, o.slug AS org_slug, o.name AS org_name, f.slug, f.name, f.description,
			0 AS depth,
			(
				WITH RECURSIVE down AS (
					SELECT f.id
					UNION ALL
					SELECT c.id
					FROM mcp_folders c
					JOIN down ON c.parent_id = down.id
				)
				SELECT count(*)
				FROM mcp_servers k
				WHERE k.folder_id IN (SELECT id FROM down)
					AND (k.status <> 'draft' OR (k.organization_id IS NULL AND $5) OR k.organization_id = ANY($4))
			) AS servers,
			(SELECT count(*) FROM mcp_folders c WHERE c.parent_id = f.id) AS folders,
			rfc3339(f.created_at) AS created_at, rfc3339(f.updated_at) AS updated_at
		FROM mcp_folders f
		LEFT JOIN nodes o ON o.id = f.organization_id
		WHERE f.parent_id IS NOT DISTINCT FROM $1
			AND ((f.organization_id IS NULL AND $3) OR f.organization_id = ANY($2))
		ORDER BY lower(f.name), f.id`, parent, v.Organizations, v.Global, w.Organizations, w.Global)
}

func (s *FolderRepository) Insert(ctx context.Context, f gatewaytypes.NewFolder) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO mcp_folders (id, parent_id, organization_id, slug, name, description)
		VALUES ($1, $2, $3, $4, $5, $6)`, f.ID, f.ParentID, f.OrganizationID, f.Slug, f.Name, f.Description)
	return err
}

func (s *FolderRepository) Update(ctx context.Context, id uuid.UUID, slug, name, description string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE mcp_folders
		SET slug = $2, name = $3, description = $4, updated_at = now()
		WHERE id = $1`, id, slug, name, description)
}

func (s *FolderRepository) Move(ctx context.Context, id uuid.UUID, parent, organization *uuid.UUID) error {
	if err := postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE mcp_folders
		SET parent_id = $2, updated_at = now()
		WHERE id = $1`, id, parent); err != nil {
		return err
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH RECURSIVE down AS (
			SELECT id
			FROM mcp_folders
			WHERE id = $1
			UNION ALL
			SELECT c.id
			FROM mcp_folders c
			JOIN down ON c.parent_id = down.id
		),
		folders AS (
			UPDATE mcp_folders
			SET organization_id = $2
			WHERE id IN (SELECT id FROM down)
				AND organization_id IS DISTINCT FROM $2
		),
		servers AS (
			UPDATE mcp_servers
			SET organization_id = $2
			WHERE folder_id IN (SELECT id FROM down)
				AND organization_id IS DISTINCT FROM $2
			RETURNING id
		),
		sets AS (
			DELETE FROM mcp_organization_servers
			WHERE server_id IN (SELECT id FROM servers)
				AND ($2::uuid IS NULL OR organization_id <> $2)
		)
		UPDATE mcp_server_checks
		SET next_check_at = now()
		WHERE server_id IN (SELECT k.id FROM mcp_servers k WHERE k.folder_id IN (SELECT id FROM down))`, id, organization)
	return err
}

func (s *FolderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM mcp_folders
		WHERE id = $1`, id)
}

func (s *FolderRepository) HasContent(ctx context.Context, id uuid.UUID) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT true
		WHERE EXISTS (SELECT 1 FROM mcp_folders WHERE parent_id = $1)
			OR EXISTS (SELECT 1 FROM mcp_servers WHERE folder_id = $1)`, id)
	return found != nil, err
}

func (s *FolderRepository) Height(ctx context.Context, id uuid.UUID) (int, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int], `
		WITH RECURSIVE down AS (
			SELECT id, 0 AS lvl
			FROM mcp_folders
			WHERE id = $1
			UNION ALL
			SELECT c.id, down.lvl + 1
			FROM mcp_folders c
			JOIN down ON c.parent_id = down.id
		)
		SELECT max(lvl)
		FROM down`, id)
}

func (s *FolderRepository) Lock(ctx context.Context) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		SELECT pg_advisory_xact_lock(hashtext('mcp_folders'))`)
	return err
}

func (s *FolderRepository) IsOrganization(ctx context.Context, id uuid.UUID) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT true
		FROM nodes
		WHERE id = $1
			AND parent_id IS NULL
			AND kind = 'organization'`, id)
	return found != nil, err
}

func (s *FolderRepository) Organizations(ctx context.Context, ids []uuid.UUID) ([]gatewaytypes.Ref, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(refRow.ref), `
		SELECT id, slug, name
		FROM nodes
		WHERE id = ANY($1)
			AND parent_id IS NULL
		ORDER BY lower(name), id`, ids)
}
