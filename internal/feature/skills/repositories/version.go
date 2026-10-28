package repositories

import (
	"context"

	"github.com/google/uuid"

	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type VersionRepository struct{ db *pgtx.DB }

func NewVersionRepository(db *pgtx.DB) *VersionRepository { return &VersionRepository{db: db} }

func (s *VersionRepository) Of(ctx context.Context, skills []uuid.UUID) (map[uuid.UUID][]skillstypes.Version, error) {
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(versionRow.version), `
		SELECT v.id, v.skill_id, v.version, rfc3339(v.created_at) AS created_at, v.origin, v.commit_sha, v.tag,
			u.id AS created_by_id, u.display_name AS created_by_name, v.files, v.size, v.digest, v.yanked, v.manifest
		FROM skill_versions v
		LEFT JOIN users u ON u.id = v.created_by
		WHERE v.skill_id = ANY($1)
		ORDER BY v.skill_id, v.major DESC, v.minor DESC, v.patch DESC, v.prerelease = '' DESC, v.created_at DESC`, skills)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]skillstypes.Version{}
	for _, v := range rows {
		out[v.SkillID] = append(out[v.SkillID], v)
	}
	return out, nil
}

type NewVersion struct {
	ID        uuid.UUID
	SkillID   uuid.UUID
	Version   string
	Parts     skillstypes.Parts
	Origin    skillstypes.Origin
	Commit    *string
	Tag       *string
	Digest    string
	Files     []skillstypes.File
	Size      int64
	Manifest  skillstypes.Manifest
	CreatedBy *uuid.UUID
}

func (s *VersionRepository) Insert(ctx context.Context, v NewVersion) error {
	if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO skill_versions (id, skill_id, version, major, minor, patch, prerelease, origin, commit_sha, tag, digest,
			files, size, manifest, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		v.ID, v.SkillID, v.Version, v.Parts.Major, v.Parts.Minor, v.Parts.Patch, v.Parts.Prerelease, string(v.Origin), v.Commit,
		v.Tag, v.Digest, len(v.Files), v.Size, v.Manifest, v.CreatedBy); err != nil {
		return err
	}
	paths, sizes, sums, kinds, types, titles := make([]string, len(v.Files)), make([]int64, len(v.Files)),
		make([][]byte, len(v.Files)), make([]string, len(v.Files)), make([]string, len(v.Files)), make([]*string, len(v.Files))
	for i, f := range v.Files {
		paths[i], sizes[i], sums[i], kinds[i], types[i], titles[i] = f.Path, f.Size, f.SHA256, string(f.Kind), f.ContentType, f.Title
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO skill_files (version_id, path, size, sha256, kind, content_type, title)
		SELECT $1, f.path, f.size, f.sha256, f.kind, f.content_type, f.title
		FROM unnest($2::text[], $3::bigint[], $4::bytea[], $5::text[], $6::text[], $7::text[])
			AS f(path, size, sha256, kind, content_type, title)`, v.ID, paths, sizes, sums, kinds, types, titles)
	return err
}

func (s *VersionRepository) Files(ctx context.Context, versionID uuid.UUID) ([]skillstypes.File, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(fileRow.file), `
		SELECT path, size, sha256, kind, content_type, title
		FROM skill_files
		WHERE version_id = $1
		ORDER BY path COLLATE "C"`, versionID)
}

func (s *VersionRepository) Delete(ctx context.Context, ids []uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM skill_versions
		WHERE id = ANY($1)`, ids)
	return err
}

func (s *VersionRepository) Yank(ctx context.Context, skillID uuid.UUID, version string, yanked bool) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_versions
		SET yanked = $3
		WHERE skill_id = $1
			AND version = $2`, skillID, version, yanked)
}

func (s *VersionRepository) Policies(ctx context.Context, skills, organizations []uuid.UUID) (map[uuid.UUID]skillstypes.Policy, error) {
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(func(r ownedPolicyRow) ownedPolicyRow { return r }), `
		SELECT COALESCE(skill_id, organization_id) AS owner, version_range, prerelease, excluded
		FROM skill_version_policies
		WHERE skill_id = ANY($1)
			OR organization_id = ANY($2)`, skills, organizations)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]skillstypes.Policy, len(rows))
	for _, r := range rows {
		out[r.Owner] = r.policy()
	}
	return out, nil
}

type ownedPolicyRow struct {
	Owner uuid.UUID `db:"owner"`
	policyRow
}

func (s *VersionRepository) PutPolicy(ctx context.Context, id uuid.UUID, skill, organization *uuid.UUID, p skillstypes.Policy) error {
	if p.Range == nil && p.Prerelease == nil && p.Exclude == nil {
		_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
			DELETE FROM skill_version_policies
			WHERE skill_id IS NOT DISTINCT FROM $1
				AND organization_id IS NOT DISTINCT FROM $2`, skill, organization)
		return err
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH updated AS (
			UPDATE skill_version_policies
			SET version_range = $4, prerelease = $5, excluded = $6, updated_at = now()
			WHERE skill_id IS NOT DISTINCT FROM $2
				AND organization_id IS NOT DISTINCT FROM $3
			RETURNING id
		)
		INSERT INTO skill_version_policies (id, skill_id, organization_id, version_range, prerelease, excluded)
		SELECT $1, $2, $3, $4, $5, $6
		WHERE NOT EXISTS (SELECT 1 FROM updated)`, id, skill, organization, p.Range, p.Prerelease, p.Exclude)
	return err
}
