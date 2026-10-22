package repositories

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/postgres"
)

type WorkloadRepository struct{ db *postgres.DB }

func NewWorkloadRepository(db *postgres.DB) *WorkloadRepository { return &WorkloadRepository{db: db} }

func (s *WorkloadRepository) Previous(ctx context.Context, clusterID uuid.UUID) (map[string]deploytypes.Previous, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT uid, version, pending_version, pending_since, state
		FROM cluster_workloads
		WHERE cluster_id = $1`, clusterID)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	defer rows.Close()
	out := map[string]deploytypes.Previous{}
	for rows.Next() {
		var uid string
		var p deploytypes.Previous
		if err := rows.Scan(&uid, &p.Version, &p.PendingVersion, &p.PendingSince, &p.State); err != nil {
			return nil, postgres.Translate(err, constraints)
		}
		out[uid] = p
	}
	return out, postgres.Translate(rows.Err(), constraints)
}

func (s *WorkloadRepository) Save(ctx context.Context, clusterID uuid.UUID, obs []deploytypes.Observation) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		uids := make([]string, 0, len(obs))
		for _, o := range obs {
			uids = append(uids, o.UID)
			var reason *string
			if o.Reason != nil {
				r := string(*o.Reason)
				reason = &r
			}
			images := o.Images
			if images == nil {
				images = []deploytypes.Image{}
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO cluster_workloads (id, cluster_id, uid, namespace, kind, name, project_id, reason,
					annotation, service, environment, branch, version, images, state, pending_version, pending_since)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15::jsonb, $16, $17)
				ON CONFLICT (cluster_id, uid)
				DO UPDATE SET namespace = EXCLUDED.namespace, kind = EXCLUDED.kind, name = EXCLUDED.name,
					project_id = EXCLUDED.project_id, reason = EXCLUDED.reason, annotation = EXCLUDED.annotation,
					service = EXCLUDED.service, environment = EXCLUDED.environment, branch = EXCLUDED.branch,
					version = EXCLUDED.version, images = EXCLUDED.images, state = EXCLUDED.state,
					pending_version = EXCLUDED.pending_version, pending_since = EXCLUDED.pending_since,
					observed_at = now(), gone_at = NULL, updated_at = now()`,
				uuid.Must(uuid.NewV7()), clusterID, o.UID, o.Namespace, string(o.Kind), o.Name,
				o.ProjectID, reason, o.Annotation, o.Service, o.Environment, o.Branch, o.Version, images, o.State,
				o.PendingVersion, o.PendingSince); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE cluster_workloads
			SET gone_at = now(), pending_version = NULL, pending_since = NULL, updated_at = now()
			WHERE cluster_id = $1
				AND gone_at IS NULL
				AND NOT (uid = ANY($2::text[]))`, clusterID, uids); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM cluster_workloads w USING nodes n
			WHERE w.cluster_id = $1
				AND n.id = w.project_id
				AND NOT n.cluster_observation`, clusterID); err != nil {
			return err
		}
		return nil
	})
	return postgres.Translate(err, constraints)
}

func (s *WorkloadRepository) Unmatched(ctx context.Context, clusterID uuid.UUID, limit uint32, offset uint64) ([]deploytypes.Unmatched, uint64, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT w.namespace, w.kind, w.name, w.reason, w.annotation, rfc3339(w.observed_at) AS observed_at
		FROM cluster_workloads w
		WHERE w.cluster_id = $1
			AND w.project_id IS NULL
			AND w.gone_at IS NULL
		ORDER BY w.namespace, w.kind, w.name
		LIMIT $2
		OFFSET $3`, clusterID, int64(limit), int64(offset))
	if err != nil {
		return nil, 0, postgres.Translate(err, constraints)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (deploytypes.Unmatched, error) {
		var u deploytypes.Unmatched
		var kind, reason string
		err := r.Scan(&u.Namespace, &kind, &u.Name, &reason, &u.Annotation, &u.ObservedAt)
		u.Kind, u.Reason = deploytypes.Kind(kind), deploytypes.Reason(reason)
		return u, err
	})
	if err != nil {
		return nil, 0, postgres.Translate(err, constraints)
	}
	var total int64
	if err := s.db.From(ctx).QueryRow(ctx, `
		SELECT count(*)
		FROM cluster_workloads w
		WHERE w.cluster_id = $1
			AND w.project_id IS NULL
			AND w.gone_at IS NULL`, clusterID).Scan(&total); err != nil {
		return nil, 0, postgres.Translate(err, constraints)
	}
	return items, uint64(max(total, 0)), nil
}

func (s *WorkloadRepository) ForProject(ctx context.Context, projectID uuid.UUID) ([]deploytypes.Observed, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT c.name AS cluster, w.namespace, w.kind, w.name, w.service, w.environment, w.branch,
			w.version, w.images, w.state, rfc3339(w.observed_at) AS observed_at, w.gone_at IS NOT NULL AS gone
		FROM cluster_workloads w
		JOIN clusters c ON c.id = w.cluster_id
		JOIN nodes n ON n.id = w.project_id
		WHERE w.project_id = $1
			AND n.cluster_observation
		ORDER BY w.service, w.environment, c.name, w.namespace, w.name`, projectID)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (deploytypes.Observed, error) {
		var o deploytypes.Observed
		var kind string
		err := r.Scan(&o.Cluster, &o.Namespace, &kind, &o.Name, &o.Service, &o.Environment, &o.Branch, &o.Version,
			&o.Images, &o.State, &o.ObservedAt, &o.Gone)
		o.Kind = deploytypes.Kind(kind)
		return o, err
	})
	return items, postgres.Translate(err, constraints)
}

func (s *WorkloadRepository) Prune(ctx context.Context, days uint32) (int64, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM cluster_workloads
		WHERE gone_at < now() - make_interval(days => $1::int)`, int(days))
	if err != nil {
		return 0, postgres.Translate(err, constraints)
	}
	return tag.RowsAffected(), nil
}

func (s *WorkloadRepository) Project(ctx context.Context, ref string) (*deploytypes.ProjectRef, error) {
	if id, err := uuid.Parse(ref); err == nil {
		var p deploytypes.ProjectRef
		err := s.db.From(ctx).QueryRow(ctx, `
			SELECT id, cluster_observation
			FROM nodes
			WHERE id = $1
				AND kind = 'project'`, id).Scan(&p.ID, &p.Observe)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, postgres.Translate(err, constraints)
		}
		return &p, nil
	}
	var parent *uuid.UUID
	segments := strings.Split(ref, "/")
	for i, seg := range segments {
		var id uuid.UUID
		var kind string
		var observe bool
		err := s.db.From(ctx).QueryRow(ctx, `
			SELECT id, kind, cluster_observation
			FROM nodes
			WHERE parent_id IS NOT DISTINCT FROM $1
				AND slug = $2`, parent, strings.ToLower(seg)).Scan(&id, &kind, &observe)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, postgres.Translate(err, constraints)
		}
		if i == len(segments)-1 {
			if kind != "project" {
				return nil, nil
			}
			return &deploytypes.ProjectRef{ID: id, Observe: observe}, nil
		}
		parent = &id
	}
	return nil, nil
}

func (s *WorkloadRepository) Current(ctx context.Context, projectID uuid.UUID, service, environment string) (*string, error) {
	var v string
	err := s.db.From(ctx).QueryRow(ctx, `
		SELECT d.version
		FROM service_environments e
		JOIN service_deployments d ON d.id = e.deployment_id
		WHERE e.project_id = $1
			AND e.service = $2
			AND e.environment = $3`, projectID, service, environment).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	return &v, nil
}
