package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/platform/postgres"
)

type Workloads struct{ db *postgres.DB }

func NewWorkloads(db *postgres.DB) *Workloads { return &Workloads{db: db} }

func (s *Workloads) Previous(ctx context.Context, clusterID uuid.UUID) (map[string]deploy.Previous, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT uid, version, pending_version, pending_since, state
		FROM cluster_workloads
		WHERE cluster_id = $1`, clusterID)
	if err != nil {
		return nil, dbErr(err)
	}
	defer rows.Close()
	out := map[string]deploy.Previous{}
	for rows.Next() {
		var uid string
		var p deploy.Previous
		if err := rows.Scan(&uid, &p.Version, &p.PendingVersion, &p.PendingSince, &p.State); err != nil {
			return nil, dbErr(err)
		}
		out[uid] = p
	}
	return out, dbErr(rows.Err())
}

func (s *Workloads) Save(ctx context.Context, clusterID uuid.UUID, obs []deploy.Observation) error {
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
				images = []deploy.Image{}
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
	return dbErr(err)
}

func (s *Workloads) Unmatched(ctx context.Context, clusterID uuid.UUID, limit uint32, offset uint64) ([]deploy.Unmatched, uint64, error) {
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
		return nil, 0, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (deploy.Unmatched, error) {
		var u deploy.Unmatched
		var kind, reason string
		err := r.Scan(&u.Namespace, &kind, &u.Name, &reason, &u.Annotation, &u.ObservedAt)
		u.Kind, u.Reason = deploy.Kind(kind), deploy.Reason(reason)
		return u, err
	})
	if err != nil {
		return nil, 0, dbErr(err)
	}
	var total int64
	if err := s.db.From(ctx).QueryRow(ctx, `
		SELECT count(*)
		FROM cluster_workloads w
		WHERE w.cluster_id = $1
			AND w.project_id IS NULL
			AND w.gone_at IS NULL`, clusterID).Scan(&total); err != nil {
		return nil, 0, dbErr(err)
	}
	return items, uint64(max(total, 0)), nil
}

func (s *Workloads) ForProject(ctx context.Context, projectID uuid.UUID) ([]deploy.Observed, error) {
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
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (deploy.Observed, error) {
		var o deploy.Observed
		var kind string
		err := r.Scan(&o.Cluster, &o.Namespace, &kind, &o.Name, &o.Service, &o.Environment, &o.Branch, &o.Version,
			&o.Images, &o.State, &o.ObservedAt, &o.Gone)
		o.Kind = deploy.Kind(kind)
		return o, err
	})
	return items, dbErr(err)
}

func (s *Workloads) Prune(ctx context.Context, days uint32) (int64, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM cluster_workloads
		WHERE gone_at < now() - make_interval(days => $1::int)`, int(days))
	if err != nil {
		return 0, dbErr(err)
	}
	return tag.RowsAffected(), nil
}

func (s *Workloads) Project(ctx context.Context, ref string) (*deploy.ProjectRef, error) {
	if id, err := uuid.Parse(ref); err == nil {
		var p deploy.ProjectRef
		err := s.db.From(ctx).QueryRow(ctx, `
			SELECT id, cluster_observation
			FROM nodes
			WHERE id = $1
				AND kind = 'project'`, id).Scan(&p.ID, &p.Observe)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, dbErr(err)
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
			return nil, dbErr(err)
		}
		if i == len(segments)-1 {
			if kind != "project" {
				return nil, nil
			}
			return &deploy.ProjectRef{ID: id, Observe: observe}, nil
		}
		parent = &id
	}
	return nil, nil
}

func (s *Workloads) Current(ctx context.Context, projectID uuid.UUID, service, environment string) (*string, error) {
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
		return nil, dbErr(err)
	}
	return &v, nil
}
