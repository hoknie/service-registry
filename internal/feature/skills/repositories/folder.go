package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type FolderRepository struct{ db *pgtx.DB }

func NewFolderRepository(db *pgtx.DB) *FolderRepository { return &FolderRepository{db: db} }

func (s *FolderRepository) Chain(ctx context.Context, id uuid.UUID) ([]skillstypes.Folder, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(folderRow.folder), `
		WITH RECURSIVE up AS (
			SELECT f.*, 0 AS lvl
			FROM skill_folders f
			WHERE f.id = $1
			UNION ALL
			SELECT p.*, up.lvl + 1
			FROM skill_folders p
			JOIN up ON p.id = up.parent_id
		)
		SELECT up.id, up.parent_id, up.organization_id, o.slug AS org_slug, o.name AS org_name, up.slug, up.name,
			up.description, up.lvl AS depth, 0::bigint AS skills, 0::bigint AS folders,
			rfc3339(up.created_at) AS created_at, rfc3339(up.updated_at) AS updated_at, up.disabled, up.source_id,
			dis.id AS disabled_by_id, dis.name AS disabled_by_name, dis.slug AS disabled_by_slug
		FROM up
		LEFT JOIN nodes o ON o.id = up.organization_id
		LEFT JOIN skill_folders dis ON dis.id = skill_folder_disabled(up.parent_id)
		ORDER BY up.lvl`, id)
}

func (s *FolderRepository) List(ctx context.Context, parent *uuid.UUID, v skillstypes.Visible, w skillstypes.Visible) ([]skillstypes.Folder, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(folderRow.folder), `
		SELECT f.id, f.parent_id, f.organization_id, o.slug AS org_slug, o.name AS org_name, f.slug, f.name, f.description,
			0 AS depth,
			(
				WITH RECURSIVE down AS (
					SELECT f.id
					UNION ALL
					SELECT c.id
					FROM skill_folders c
					JOIN down ON c.parent_id = down.id
				)
				SELECT count(*)
				FROM skills k
				WHERE k.folder_id IN (SELECT id FROM down)
					AND (k.status <> 'draft'
						OR (k.organization_id IS NULL AND $5)
						OR k.organization_id = ANY($4))
			) AS skills,
			(SELECT count(*) FROM skill_folders c WHERE c.parent_id = f.id) AS folders,
			rfc3339(f.created_at) AS created_at, rfc3339(f.updated_at) AS updated_at, f.disabled, f.source_id,
			dis.id AS disabled_by_id, dis.name AS disabled_by_name, dis.slug AS disabled_by_slug
		FROM skill_folders f
		LEFT JOIN nodes o ON o.id = f.organization_id
		LEFT JOIN skill_folders dis ON dis.id = skill_folder_disabled(f.parent_id)
		WHERE f.parent_id IS NOT DISTINCT FROM $1
			AND ((f.organization_id IS NULL AND $3) OR f.organization_id = ANY($2))
		ORDER BY lower(f.name), f.id`, parent, v.Organizations, v.Global, w.Organizations, w.Global)
}

func (s *FolderRepository) Insert(ctx context.Context, f skillstypes.NewFolder) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO skill_folders (id, parent_id, organization_id, slug, name, description)
		VALUES ($1, $2, $3, $4, $5, $6)`, f.ID, f.ParentID, f.OrganizationID, f.Slug, f.Name, f.Description)
	return err
}

func (s *FolderRepository) Update(ctx context.Context, id uuid.UUID, slug, name, description string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_folders
		SET slug = $2, name = $3, description = $4, updated_at = now()
		WHERE id = $1`, id, slug, name, description)
}

func (s *FolderRepository) Move(ctx context.Context, id uuid.UUID, parent, organization *uuid.UUID) error {
	if err := postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_folders
		SET parent_id = $2, updated_at = now()
		WHERE id = $1`, id, parent); err != nil {
		return err
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH RECURSIVE down AS (
			SELECT id
			FROM skill_folders
			WHERE id = $1
			UNION ALL
			SELECT c.id
			FROM skill_folders c
			JOIN down ON c.parent_id = down.id
		),
		folders AS (
			UPDATE skill_folders
			SET organization_id = $2
			WHERE id IN (SELECT id FROM down)
				AND organization_id IS DISTINCT FROM $2
		)
		UPDATE skills
		SET organization_id = $2
		WHERE folder_id IN (SELECT id FROM down)
			AND organization_id IS DISTINCT FROM $2`, id, organization)
	return err
}

func (s *FolderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM skill_folders
		WHERE id = $1`, id)
}

func (s *FolderRepository) HasContent(ctx context.Context, id uuid.UUID) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT true
		WHERE EXISTS (SELECT 1 FROM skill_folders WHERE parent_id = $1)
			OR EXISTS (SELECT 1 FROM skills WHERE folder_id = $1)
			OR EXISTS (SELECT 1 FROM skill_sources WHERE folder_id = $1)`, id)
	return found != nil, err
}

func (s *FolderRepository) Height(ctx context.Context, id uuid.UUID) (int, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int], `
		WITH RECURSIVE down AS (
			SELECT id, 0 AS lvl
			FROM skill_folders
			WHERE id = $1
			UNION ALL
			SELECT c.id, down.lvl + 1
			FROM skill_folders c
			JOIN down ON c.parent_id = down.id
		)
		SELECT max(lvl)
		FROM down`, id)
}

func (s *FolderRepository) Lock(ctx context.Context) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		SELECT pg_advisory_xact_lock(hashtext('skill_folders'))`)
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

func (s *FolderRepository) Organizations(ctx context.Context, ids []uuid.UUID) ([]skillstypes.Ref, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (skillstypes.Ref, error) {
		var ref skillstypes.Ref
		err := r.Scan(&ref.ID, &ref.Slug, &ref.Name)
		return ref, err
	}, `
		SELECT id, slug, name
		FROM nodes
		WHERE id = ANY($1)
			AND parent_id IS NULL
		ORDER BY lower(name), id`, ids)
}

func (s *FolderRepository) SourceFolders(ctx context.Context, source uuid.UUID) (map[string]uuid.UUID, error) {
	return postgres.Pairs[string, uuid.UUID](ctx, s.db.From(ctx), constraints, `
		SELECT source_path, id
		FROM skill_folders
		WHERE source_id = $1`, source)
}

func (s *FolderRepository) InsertSource(ctx context.Context, f skillstypes.NewFolder, source uuid.UUID, key string) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		INSERT INTO skill_folders (id, parent_id, organization_id, slug, name, description, source_id, source_path)
		VALUES ($1, $2, $3, $4, $5, '', $6, $7)
		ON CONFLICT DO NOTHING
		RETURNING true`, f.ID, f.ParentID, f.OrganizationID, f.Slug, f.Name, source, key)
	return found != nil, err
}

func (s *FolderRepository) DropEmpty(ctx context.Context, source uuid.UUID) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM skill_folders f
		WHERE f.source_id = $1
			AND NOT EXISTS (SELECT 1 FROM skill_folders c WHERE c.parent_id = f.id)
			AND NOT EXISTS (SELECT 1 FROM skills k WHERE k.folder_id = f.id)
			AND NOT EXISTS (SELECT 1 FROM skill_sources x WHERE x.folder_id = f.id)`, source)
}

func (s *FolderRepository) SetDisabled(ctx context.Context, id uuid.UUID, disabled bool) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_folders
		SET disabled = $2, updated_at = now()
		WHERE id = $1`, id, disabled)
}
