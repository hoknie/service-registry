package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/postgres"
)

type Scans struct{ db *postgres.DB }

func NewScans(db *postgres.DB) *Scans { return &Scans{db: db} }

func (s *Scans) Record(ctx context.Context, scan knowledge.Scan, keep int, maxGap time.Duration) error {
	branches, err := json.Marshal(nonNil(scan.Branches))
	if err != nil {
		return dbErr(err)
	}
	var index []byte
	if scan.Index != nil {
		if index, err = json.Marshal(scan.Index); err != nil {
			return dbErr(err)
		}
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
		if scan.Status == knowledge.ScanUnchanged {
			var last knowledge.ScanTail
			var status, trigger string
			var lastBranches []byte
			err := tx.QueryRow(ctx, `
				SELECT id, status, trigger, branches, finished_at
				FROM knowledge_scans
				WHERE project_id = $1
					AND kind = $2
				ORDER BY last_started_at DESC, id DESC
				LIMIT 1
				FOR UPDATE`, scan.ProjectID, string(scan.Kind)).Scan(&last.ID, &status, &trigger, &lastBranches, &last.FinishedAt)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil {
				last.Status, last.Trigger = knowledge.ScanStatus(status), knowledge.ScanTrigger(trigger)
				if err := json.Unmarshal(lastBranches, &last.Branches); err != nil {
					return err
				}
				if scan.Extends(last, maxGap) {
					_, err := tx.Exec(ctx, `
						UPDATE knowledge_scans
						SET repeats = repeats + 1, finished_at = $2, last_started_at = $3, duration_ms = $4
						WHERE id = $1`, last.ID, scan.FinishedAt, scan.StartedAt, scan.DurationMS())
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_scans (id, project_id, kind, trigger, source, status, started_at, finished_at,
				last_started_at, duration_ms, branches, index, error_code, error_detail, warnings)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $7, $9, $10, $11, $12, $13, $14)`,
			scan.ID, scan.ProjectID, string(scan.Kind), string(scan.Trigger), source, string(scan.Status), scan.StartedAt,
			scan.FinishedAt, scan.DurationMS(), branches, index, code, detail, nonNil(scan.Warnings)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
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
	return dbErr(err)
}

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

func (s *Scans) List(ctx context.Context, f knowledge.ScanFilter, page access.PageRequest) (access.Page[knowledge.ScanItem], error) {
	var kind, trigger *string
	if f.Kind != nil {
		k := string(*f.Kind)
		kind = &k
	}
	if f.Trigger != nil {
		t := string(*f.Trigger)
		trigger = &t
	}
	var statuses []string
	for _, st := range f.Statuses {
		statuses = append(statuses, string(st))
	}
	rows, err := s.db.From(ctx).Query(ctx, `
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
			k.error_code, k.error_detail, k.warnings, count(*) OVER ()
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
		return access.Page[knowledge.ScanItem]{}, dbErr(err)
	}
	out := access.Page[knowledge.ScanItem]{Items: []knowledge.ScanItem{}, Limit: page.Limit, Offset: page.Offset}
	defer rows.Close()
	for rows.Next() {
		var it knowledge.ScanItem
		var kindRaw, triggerRaw, statusRaw string
		var branches, index []byte
		var code, detail *string
		var total int64
		if err := rows.Scan(&it.ID, &it.ProjectID, &it.ProjectName, &it.ProjectPath, &kindRaw, &triggerRaw, &it.Source, &statusRaw,
			&it.StartedAt, &it.LastStartedAt, &it.FinishedAt, &it.DurationMS, &it.Repeats, &branches, &index, &code, &detail, &it.Warnings, &total); err != nil {
			return access.Page[knowledge.ScanItem]{}, dbErr(err)
		}
		it.Kind, it.Trigger, it.Status = knowledge.ScanKind(kindRaw), knowledge.ScanTrigger(triggerRaw), knowledge.ScanStatus(statusRaw)
		if err := json.Unmarshal(branches, &it.Branches); err != nil {
			return access.Page[knowledge.ScanItem]{}, dbErr(err)
		}
		if index != nil {
			it.Index = &knowledge.ScanIndex{}
			if err := json.Unmarshal(index, it.Index); err != nil {
				return access.Page[knowledge.ScanItem]{}, dbErr(err)
			}
		}
		if code != nil {
			it.Error = &knowledge.ScanError{Code: *code}
			if detail != nil {
				it.Error.Detail = *detail
			}
		}
		out.Total = uint64(total)
		out.Items = append(out.Items, it)
	}
	if err := rows.Err(); err != nil {
		return access.Page[knowledge.ScanItem]{}, dbErr(err)
	}
	if len(out.Items) == 0 && page.Offset > 0 {
		var total int64
		if err := s.db.From(ctx).QueryRow(ctx, `
			SELECT count(*)
			FROM knowledge_scans k
			WHERE ($1::uuid IS NULL OR k.project_id = $1)
				AND ($2::text IS NULL OR k.kind = $2)
				AND ($3::text[] IS NULL OR k.status = ANY($3))
				AND ($4::text IS NULL OR k.trigger = $4)`, f.Project, kind, statuses, trigger).Scan(&total); err != nil {
			return access.Page[knowledge.ScanItem]{}, dbErr(err)
		}
		out.Total = uint64(total)
	}
	return out, nil
}
