package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SetRepository struct{ db *pgtx.DB }

func NewSetRepository(db *pgtx.DB) *SetRepository { return &SetRepository{db: db} }

func (s *SetRepository) Get(ctx context.Context, organization uuid.UUID) (skillstypes.Set, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (skillstypes.Set, error) {
		var set skillstypes.Set
		err := r.Scan(&set.Mode, &set.Folders, &set.Skills)
		return set, err
	}, `
		SELECT mode, folder_ids, skill_ids
		FROM skill_organization_sets
		WHERE organization_id = $1`, organization)
	if err != nil || found == nil {
		return skillstypes.Set{Mode: skillstypes.SetAll, Folders: []uuid.UUID{}, Skills: []uuid.UUID{}}, err
	}
	return *found, nil
}

func (s *SetRepository) Put(ctx context.Context, organization uuid.UUID, set skillstypes.Set) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO skill_organization_sets (organization_id, mode, folder_ids, skill_ids)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (organization_id)
		DO UPDATE SET mode = $2, folder_ids = $3, skill_ids = $4, updated_at = now()`,
		organization, set.Mode, set.Folders, set.Skills)
	return err
}

func (s *SetRepository) Describe(ctx context.Context, folders, skills []uuid.UUID) ([]skillstypes.SetItem, []skillstypes.SetItem, error) {
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (skillstypes.SetItem, error) {
		var it skillstypes.SetItem
		var status string
		err := r.Scan(&it.Kind, &it.ID, &it.Path, &it.Name, &it.Skills, &status, &it.Global)
		it.Status = skillstypes.Status(status)
		return it, err
	}, `
		WITH RECURSIVE fp AS (
			SELECT id, slug::text AS path
			FROM skill_folders
			WHERE parent_id IS NULL
			UNION ALL
			SELECT f.id, fp.path || '/' || f.slug
			FROM skill_folders f
			JOIN fp ON f.parent_id = fp.id
		)
		SELECT 'folder', f.id, fp.path, f.name,
			(SELECT count(*) FROM skills k WHERE f.id = ANY(skill_folder_ancestors(k.folder_id)) AND k.status <> 'draft'),
			'', f.organization_id IS NULL
		FROM skill_folders f
		JOIN fp ON fp.id = f.id
		WHERE f.id = ANY($1)
		UNION ALL
		SELECT 'skill', k.id, COALESCE(fp.path || '/', '') || k.slug, k.title, 0, k.status, k.organization_id IS NULL
		FROM skills k
		LEFT JOIN fp ON fp.id = k.folder_id
		WHERE k.id = ANY($2)
		ORDER BY 3`, folders, skills)
	if err != nil {
		return nil, nil, err
	}
	var outFolders, outSkills []skillstypes.SetItem
	for _, it := range items {
		if it.Kind == "skill" {
			outSkills = append(outSkills, it)
			continue
		}
		outFolders = append(outFolders, it)
	}
	return outFolders, outSkills, nil
}
