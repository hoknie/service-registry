package repositories

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ScanRepository struct{ db *pgtx.DB }

func NewScanRepository(db *pgtx.DB) *ScanRepository { return &ScanRepository{db: db} }

func (s *ScanRepository) LockLast(ctx context.Context, scan knowledgetypes.Scan) (*knowledgetypes.ScanTail, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanTail, `
		SELECT id, status, trigger, branches, finished_at
		FROM knowledge_scans
		WHERE project_id = $1
			AND kind = $2
		ORDER BY last_started_at DESC, id DESC
		LIMIT 1
		FOR UPDATE`, scan.ProjectID, string(scan.Kind))
}

func (s *ScanRepository) Repeat(ctx context.Context, id uuid.UUID, scan knowledgetypes.Scan) error {
	notes, err := json.Marshal(scan.Notes)
	if err != nil {
		return err
	}
	counts, err := json.Marshal(scan.Counts)
	if err != nil {
		return err
	}
	_, err = postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_scans
		SET repeats = repeats + 1, finished_at = $2, last_started_at = $3, duration_ms = $4,
			warning_details = $5, warning_counts = $6
		WHERE id = $1`, id, scan.FinishedAt, scan.StartedAt, scan.DurationMS(), notes, counts)
	return err
}

func (s *ScanRepository) Insert(ctx context.Context, scan knowledgetypes.Scan) error {
	a, err := scanArgsOf(scan)
	if err != nil {
		return err
	}
	_, err = postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO knowledge_scans (id, project_id, kind, trigger, source, status, started_at, finished_at,
			last_started_at, duration_ms, branches, index, error_code, error_detail, warnings, warning_details, warning_counts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $7, $9, $10, $11, $12, $13, $14, $15, $16)`,
		scan.ID, scan.ProjectID, string(scan.Kind), string(scan.Trigger), a.source, string(scan.Status), scan.StartedAt,
		scan.FinishedAt, scan.DurationMS(), a.branches, a.index, a.code, a.detail, scan.Warnings, a.notes, a.counts)
	return err
}

func (s *ScanRepository) Trim(ctx context.Context, scan knowledgetypes.Scan, keep int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM knowledge_scans
		WHERE id IN (
			SELECT id
			FROM knowledge_scans
			WHERE project_id = $1
				AND kind = $2
			ORDER BY last_started_at DESC, id DESC
			OFFSET $3
		)`, scan.ProjectID, string(scan.Kind), keep)
	return err
}

func scanTail(r pgx.CollectableRow) (knowledgetypes.ScanTail, error) {
	var last knowledgetypes.ScanTail
	var status, trigger string
	var branches []byte
	if err := r.Scan(&last.ID, &status, &trigger, &branches, &last.FinishedAt); err != nil {
		return last, err
	}
	last.Status, last.Trigger = knowledgetypes.ScanStatus(status), knowledgetypes.ScanTrigger(trigger)
	return last, json.Unmarshal(branches, &last.Branches)
}

func (s *ScanRepository) List(ctx context.Context, f knowledgetypes.ScanFilter, page accesstypes.PageRequest) (accesstypes.Page[knowledgetypes.ScanItem], error) {
	kind, trigger, statuses := scanFilterArgs(f)
	var total int64
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.ScanItem, error) {
		return scanItem(r, &total)
	}, `
		SELECT k.id, k.project_id, n.name, (
			WITH RECURSIVE up AS (
				SELECT id, parent_id, slug, 0 AS lvl
				FROM nodes
				WHERE id = k.project_id
				UNION ALL
				SELECT x.id, x.parent_id, x.slug, up.lvl + 1
				FROM nodes x
				JOIN up ON x.id = up.parent_id
			)
			SELECT string_agg(slug, '/' ORDER BY lvl DESC)
			FROM up
		), k.kind, k.trigger, k.source, k.status, rfc3339(k.started_at), rfc3339(k.last_started_at), rfc3339(k.finished_at),
			k.duration_ms::bigint, k.repeats, k.branches, k.index,
			k.error_code, k.error_detail, k.warnings, k.warning_details, k.warning_counts, count(*) OVER ()
		FROM knowledge_scans k
		JOIN nodes n ON n.id = k.project_id
		WHERE ($1::uuid IS NULL OR k.project_id = $1)
			AND ($2::text IS NULL OR k.kind = $2)
			AND ($3::text[] IS NULL OR k.status = ANY($3))
			AND ($4::text IS NULL OR k.trigger = $4)
		ORDER BY k.last_started_at DESC, k.id DESC
		LIMIT $5
		OFFSET $6`, f.Project, kind, statuses, trigger, page.Limit, page.Offset)
	if err != nil {
		return accesstypes.Page[knowledgetypes.ScanItem]{}, err
	}
	return accesstypes.Page[knowledgetypes.ScanItem]{Items: items, Limit: page.Limit, Offset: page.Offset, Total: uint64(total)}, nil
}

func (s *ScanRepository) Count(ctx context.Context, f knowledgetypes.ScanFilter) (uint64, error) {
	kind, trigger, statuses := scanFilterArgs(f)
	total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM knowledge_scans k
		WHERE ($1::uuid IS NULL OR k.project_id = $1)
			AND ($2::text IS NULL OR k.kind = $2)
			AND ($3::text[] IS NULL OR k.status = ANY($3))
			AND ($4::text IS NULL OR k.trigger = $4)`, f.Project, kind, statuses, trigger)
	return uint64(total), err
}

func scanFilterArgs(f knowledgetypes.ScanFilter) (kind, trigger *string, statuses []string) {
	if f.Kind != nil {
		kind = new(string(*f.Kind))
	}
	if f.Trigger != nil {
		trigger = new(string(*f.Trigger))
	}
	for _, st := range f.Statuses {
		statuses = append(statuses, string(st))
	}
	return kind, trigger, statuses
}

func scanItem(r pgx.CollectableRow, total *int64) (knowledgetypes.ScanItem, error) {
	var it knowledgetypes.ScanItem
	var kindRaw, triggerRaw, statusRaw string
	var branches, index, notes, counts []byte
	var code, detail *string
	if err := r.Scan(&it.ID, &it.ProjectID, &it.ProjectName, &it.ProjectPath, &kindRaw, &triggerRaw, &it.Source, &statusRaw,
		&it.StartedAt, &it.LastStartedAt, &it.FinishedAt, &it.DurationMS, &it.Repeats, &branches, &index, &code, &detail, &it.Warnings,
		&notes, &counts, total); err != nil {
		return it, err
	}
	if err := json.Unmarshal(notes, &it.Notes); err != nil {
		return it, err
	}
	if err := json.Unmarshal(counts, &it.Counts); err != nil {
		return it, err
	}
	it.Kind, it.Trigger, it.Status = knowledgetypes.ScanKind(kindRaw), knowledgetypes.ScanTrigger(triggerRaw), knowledgetypes.ScanStatus(statusRaw)
	if err := json.Unmarshal(branches, &it.Branches); err != nil {
		return it, err
	}
	if index != nil {
		it.Index = &knowledgetypes.ScanIndex{}
		if err := json.Unmarshal(index, it.Index); err != nil {
			return it, err
		}
	}
	if code != nil {
		it.Error = &knowledgetypes.ScanError{Code: *code}
		if detail != nil {
			it.Error.Detail = *detail
		}
	}
	return it, nil
}
