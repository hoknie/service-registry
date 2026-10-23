package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ScanRepository struct{ db *pgtx.DB }

func NewScanRepository(db *pgtx.DB) *ScanRepository { return &ScanRepository{db: db} }

func (s *ScanRepository) Record(ctx context.Context, scan knowledgetypes.Scan, keep int, maxGap time.Duration) error {
	branches, err := json.Marshal(nonNil(scan.Branches))
	if err != nil {
		return postgres.Translate(err, constraints)
	}
	var index []byte
	if scan.Index != nil {
		if index, err = json.Marshal(scan.Index); err != nil {
			return postgres.Translate(err, constraints)
		}
	}
	notes, err := json.Marshal(nonNil(scan.Notes))
	if err != nil {
		return postgres.Translate(err, constraints)
	}
	counts, err := json.Marshal(nonNilMap(scan.Counts))
	if err != nil {
		return postgres.Translate(err, constraints)
	}
	var code, detail *string
	if scan.Error != nil {
		code, detail = &scan.Error.Code, &scan.Error.Detail
	}
	var source *string
	if scan.Source != "" {
		source = &scan.Source
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		if scan.Status == knowledgetypes.ScanUnchanged {
			last, err := postgres.Find(ctx, tx, constraints, scanTail, `
				SELECT id, status, trigger, branches, finished_at
				FROM knowledge_scans
				WHERE project_id = $1
					AND kind = $2
				ORDER BY last_started_at DESC, id DESC
				LIMIT 1
				FOR UPDATE`, scan.ProjectID, string(scan.Kind))
			if err != nil {
				return err
			}
			if last != nil && scan.Extends(*last, maxGap) {
				_, err := postgres.Exec(ctx, tx, constraints, `
					UPDATE knowledge_scans
					SET repeats = repeats + 1, finished_at = $2, last_started_at = $3, duration_ms = $4,
						warning_details = $5, warning_counts = $6
					WHERE id = $1`, last.ID, scan.FinishedAt, scan.StartedAt, scan.DurationMS(), notes, counts)
				return err
			}
		}
		if _, err := postgres.Exec(ctx, tx, constraints, `
			INSERT INTO knowledge_scans (id, project_id, kind, trigger, source, status, started_at, finished_at,
				last_started_at, duration_ms, branches, index, error_code, error_detail, warnings, warning_details, warning_counts)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $7, $9, $10, $11, $12, $13, $14, $15, $16)`,
			scan.ID, scan.ProjectID, string(scan.Kind), string(scan.Trigger), source, string(scan.Status), scan.StartedAt,
			scan.FinishedAt, scan.DurationMS(), branches, index, code, detail, nonNil(scan.Warnings), notes, counts); err != nil {
			return err
		}
		_, err := postgres.Exec(ctx, tx, constraints, `
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
	})
	return postgres.Translate(err, constraints)
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

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

func nonNilMap(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}

func (s *ScanRepository) List(ctx context.Context, f knowledgetypes.ScanFilter, page accesstypes.PageRequest) (accesstypes.Page[knowledgetypes.ScanItem], error) {
	var kind, trigger *string
	if f.Kind != nil {
		kind = new(string(*f.Kind))
	}
	if f.Trigger != nil {
		trigger = new(string(*f.Trigger))
	}
	var statuses []string
	for _, st := range f.Statuses {
		statuses = append(statuses, string(st))
	}
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
	out := accesstypes.Page[knowledgetypes.ScanItem]{Items: items, Limit: page.Limit, Offset: page.Offset, Total: uint64(total)}
	if len(out.Items) == 0 && page.Offset > 0 {
		total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
			SELECT count(*)
			FROM knowledge_scans k
			WHERE ($1::uuid IS NULL OR k.project_id = $1)
				AND ($2::text IS NULL OR k.kind = $2)
				AND ($3::text[] IS NULL OR k.status = ANY($3))
				AND ($4::text IS NULL OR k.trigger = $4)`, f.Project, kind, statuses, trigger)
		if err != nil {
			return accesstypes.Page[knowledgetypes.ScanItem]{}, err
		}
		out.Total = uint64(total)
	}
	return out, nil
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
