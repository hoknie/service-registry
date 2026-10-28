package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SkillRepository struct{ db *pgtx.DB }

func NewSkillRepository(db *pgtx.DB) *SkillRepository { return &SkillRepository{db: db} }

type Filter struct {
	skillstypes.ListQuery
	All      bool
	Name     *string
	SourceID *uuid.UUID
}

func querySkills(ctx context.Context, q pgtx.Querier, f Filter) ([]Listed, error) {
	var status *string
	if f.Status != nil {
		status = new(string(*f.Status))
	}
	var cursorAt, cursorName *string
	var cursorID *uuid.UUID
	if f.After != nil {
		cursorAt, cursorName, cursorID = &f.After.UpdatedAt, &f.After.Name, &f.After.ID
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 1000
	}
	return postgres.List(ctx, q, constraints, postgres.Struct(listedRow.listed), `
		WITH RECURSIVE fp AS (
			SELECT id, slug::text AS path
			FROM skill_folders
			WHERE parent_id IS NULL
			UNION ALL
			SELECT f.id, fp.path || '/' || f.slug
			FROM skill_folders f
			JOIN fp ON f.parent_id = fp.id
		),
		down AS (
			SELECT id
			FROM skill_folders
			WHERE id = $5
			UNION ALL
			SELECT c.id
			FROM skill_folders c
			JOIN down ON c.parent_id = down.id
			WHERE $6
		)
		SELECT k.id, k.folder_id, k.organization_id, o.slug AS org_slug, o.name AS org_name,
			COALESCE(fp.path || '/', '') || k.slug AS path, k.slug, k.name, k.title, k.description, k.about, k.license,
			k.compatibility, k.allowed_tools, k.tags, k.author, k.image_path, k.image_sha256, k.image_type, k.status,
			k.downloads, k.source_id, src.kind AS source_kind, COALESCE(src.url, src.path) AS source_url, k.source_path,
			u.id AS created_by_id, u.display_name AS created_by_name, rfc3339(k.created_at) AS created_at,
			rfc3339(k.updated_at) AS updated_at, k.updated_at::text AS cursor_at
		FROM skills k
		LEFT JOIN fp ON fp.id = k.folder_id
		LEFT JOIN nodes o ON o.id = k.organization_id
		LEFT JOIN skill_sources src ON src.id = k.source_id
		LEFT JOIN users u ON u.id = k.created_by
		WHERE ($18 OR (((k.organization_id IS NULL AND $2) OR k.organization_id = ANY($1))
				AND (k.status = 'published' OR (k.organization_id IS NULL AND $4) OR k.organization_id = ANY($3))))
			AND ($5::uuid IS NULL OR k.folder_id IN (SELECT id FROM down))
			AND ($7::uuid IS NULL OR k.organization_id = $7)
			AND (NOT $8 OR k.organization_id IS NULL)
			AND ($9 = '' OR strpos(lower(k.name || ' ' || k.title || ' ' || k.description || ' ' ||
				array_to_string(k.tags, ' ')), lower($9)) > 0)
			AND (cardinality($10::text[]) = 0 OR k.tags @> $10)
			AND ($11::text IS NULL OR k.status = $11)
			AND ($12::uuid[] IS NULL OR k.id = ANY($12))
			AND ($13::text IS NULL OR k.name = $13)
			AND ($14::text IS NULL OR k.updated_at < $14::timestamptz
				OR (k.updated_at = $14::timestamptz AND (k.name, k.id) > ($15, $16)))
			AND ($19::uuid IS NULL OR k.source_id = $19)
			AND (NOT $20 OR k.folder_id IS NULL)
		ORDER BY k.updated_at DESC, k.name, k.id
		LIMIT $17`,
		f.Organizations, f.Global, f.Writable, f.WriteGlobal, f.FolderID, f.Recursive, f.OnlyOrg, f.OnlyGlobal, f.Query,
		nonNil(f.Tags), status, f.IDs, f.Name, cursorAt, cursorName, cursorID, limit, f.All, f.SourceID, f.TopLevel)
}

func nonNil(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func (s *SkillRepository) List(ctx context.Context, f Filter) ([]Listed, error) {
	return querySkills(ctx, s.db.From(ctx), f)
}

func (s *SkillRepository) Get(ctx context.Context, id uuid.UUID) (*skillstypes.Skill, error) {
	found, err := querySkills(ctx, s.db.From(ctx), Filter{ListQuery: skillstypes.ListQuery{IDs: []uuid.UUID{id}}, All: true})
	if err != nil || len(found) == 0 {
		return nil, err
	}
	return &found[0].Skill, nil
}

func (s *SkillRepository) LockOne(ctx context.Context, id uuid.UUID) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT id
		FROM skills
		WHERE id = $1
		FOR UPDATE`, id)
	return found != nil, err
}

type NewSkill struct {
	ID             uuid.UUID
	FolderID       *uuid.UUID
	OrganizationID *uuid.UUID
	Slug           string
	Status         skillstypes.Status
	Tags           []string
	SourceID       *uuid.UUID
	SourcePath     *string
	CreatedBy      *uuid.UUID
	Manifest       skillstypes.Manifest
}

func (s *SkillRepository) Insert(ctx context.Context, k NewSkill) error {
	m := k.Manifest
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO skills (id, folder_id, organization_id, slug, name, title, description, license, compatibility,
			allowed_tools, tags, author, image_path, status, source_id, source_path, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULLIF($13, ''), $14, $15, $16, $17)`,
		k.ID, k.FolderID, k.OrganizationID, k.Slug, m.Name, m.DisplayTitle(), m.Description, m.License, m.Compatibility,
		nonNil(m.AllowedTools), nonNil(k.Tags), m.Author, m.Image, string(k.Status), k.SourceID, k.SourcePath, k.CreatedBy)
	return err
}

func (s *SkillRepository) SetCard(ctx context.Context, id uuid.UUID, m skillstypes.Manifest, author string, tags []string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skills
		SET name = $2, title = $3, description = $4, license = $5, compatibility = $6, allowed_tools = $7, author = $8,
			image_path = NULLIF($9, ''), tags = COALESCE($10, tags), updated_at = now()
		WHERE id = $1`, id, m.Name, m.DisplayTitle(), m.Description, m.License, m.Compatibility, nonNil(m.AllowedTools),
		author, m.Image, tags)
}

func (s *SkillRepository) Patch(ctx context.Context, id uuid.UUID, status *skillstypes.Status, tags []string, slug, about *string) error {
	var st *string
	if status != nil {
		st = new(string(*status))
	}
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skills
		SET status = COALESCE($2, status), tags = COALESCE($3, tags), slug = COALESCE($4, slug), about = COALESCE($5, about),
			updated_at = now()
		WHERE id = $1`, id, st, tags, slug, about)
}

func (s *SkillRepository) Move(ctx context.Context, id uuid.UUID, folder, organization *uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skills
		SET folder_id = $2, organization_id = $3, updated_at = now()
		WHERE id = $1`, id, folder, organization)
}

func (s *SkillRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM skills
		WHERE id = $1`, id)
}

func (s *SkillRepository) CountDownload(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skills
		SET downloads = downloads + 1
		WHERE id = $1`, id)
}

func (s *SkillRepository) SetImage(ctx context.Context, id uuid.UUID, sha []byte, contentType *string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skills
		SET image_sha256 = $2, image_type = $3, updated_at = now()
		WHERE id = $1`, id, sha, contentType)
}

func (s *SkillRepository) Tags(ctx context.Context, v, w skillstypes.Visible, q string) ([]skillstypes.TagCount, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (skillstypes.TagCount, error) {
		var t skillstypes.TagCount
		err := r.Scan(&t.Tag, &t.Count)
		return t, err
	}, `
		SELECT t.tag, count(*)
		FROM skills k, unnest(k.tags) AS t(tag)
		WHERE ((k.organization_id IS NULL AND $2) OR k.organization_id = ANY($1))
			AND k.status = 'published'
			AND ($3 = '' OR strpos(t.tag, lower($3)) > 0)
		GROUP BY t.tag
		ORDER BY count(*) DESC, t.tag
		LIMIT 50`, v.Organizations, v.Global, q)
}

func (s *SkillRepository) BySource(ctx context.Context, sourceID uuid.UUID) ([]Listed, error) {
	return querySkills(ctx, s.db.From(ctx), Filter{All: true, SourceID: &sourceID})
}

func (s *SkillRepository) UserName(ctx context.Context, id uuid.UUID) (string, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		SELECT display_name
		FROM users
		WHERE id = $1`, id)
	if err != nil || found == nil {
		return "", err
	}
	return *found, nil
}
