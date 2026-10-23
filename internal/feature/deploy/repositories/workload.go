package repositories

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type WorkloadRepository struct{ db *pgtx.DB }

func NewWorkloadRepository(db *pgtx.DB) *WorkloadRepository { return &WorkloadRepository{db: db} }

func (s *WorkloadRepository) Previous(ctx context.Context, clusterID uuid.UUID) (map[string]deploytypes.Previous, error) {
	found, err := postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (previousRow, error) {
		var v previousRow
		err := r.Scan(&v.UID, &v.Version, &v.PendingVersion, &v.PendingSince, &v.State)
		return v, err
	}, `
		SELECT uid, version, pending_version, pending_since, state
		FROM cluster_workloads
		WHERE cluster_id = $1`, clusterID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]deploytypes.Previous, len(found))
	for _, v := range found {
		out[v.UID] = v.Previous
	}
	return out, nil
}

func (s *WorkloadRepository) Save(ctx context.Context, clusterID uuid.UUID, obs []deploytypes.Observation) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		rows, uids := workloadRowsOf(obs)
		for start := 0; start < len(rows); start += workloadBatch {
			if err := upsertWorkloads(ctx, tx, clusterID, rows[start:min(start+workloadBatch, len(rows))]); err != nil {
				return err
			}
		}
		if _, err := postgres.Exec(ctx, tx, constraints, `
			UPDATE cluster_workloads
			SET gone_at = now(), pending_version = NULL, pending_since = NULL, updated_at = now()
			WHERE cluster_id = $1
				AND gone_at IS NULL
				AND NOT (uid = ANY($2::text[]))`, clusterID, uids); err != nil {
			return err
		}
		if _, err := postgres.Exec(ctx, tx, constraints, `
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

const workloadBatch = 1000

func workloadRowsOf(obs []deploytypes.Observation) ([]workloadWrite, []string) {
	last := make(map[string]int, len(obs))
	for i, o := range obs {
		last[o.UID] = i
	}
	rows := make([]workloadWrite, 0, len(last))
	uids := make([]string, 0, len(last))
	for i, o := range obs {
		if last[o.UID] != i {
			continue
		}
		images := o.Images
		if images == nil {
			images = []deploytypes.Image{}
		}
		var reason *string
		if o.Reason != nil {
			reason = new(string(*o.Reason))
		}
		uids = append(uids, o.UID)
		rows = append(rows, workloadWrite{ID: uuid.Must(uuid.NewV7()), UID: o.UID, Namespace: o.Namespace,
			Kind: string(o.Kind), Name: o.Name, ProjectID: o.ProjectID, Reason: reason, Annotation: o.Annotation,
			Service: o.Service, Environment: o.Environment, Branch: o.Branch, Version: o.Version, Images: images,
			State: o.State, PendingVersion: o.PendingVersion, PendingSince: o.PendingSince})
	}
	return rows, uids
}

func upsertWorkloads(ctx context.Context, tx pgtx.Querier, clusterID uuid.UUID, rows []workloadWrite) error {
	batch, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	_, err = postgres.Exec(ctx, tx, constraints, `
		INSERT INTO cluster_workloads AS w (id, cluster_id, uid, namespace, kind, name, project_id, reason,
			annotation, service, environment, branch, version, images, state, pending_version, pending_since)
		SELECT r.id, $1, r.uid, r.namespace, r.kind, r.name, r.project_id, r.reason, r.annotation, r.service,
			r.environment, r.branch, r.version, r.images, r.state, r.pending_version, r.pending_since
		FROM jsonb_to_recordset($2::jsonb) AS r(id uuid, uid text, namespace text, kind text, name text,
			project_id uuid, reason text, annotation text, service text, environment text, branch text, version text,
			images jsonb, state jsonb, pending_version text, pending_since timestamptz)
		ON CONFLICT (cluster_id, uid)
		DO UPDATE SET namespace = EXCLUDED.namespace, kind = EXCLUDED.kind, name = EXCLUDED.name,
			version_since = CASE WHEN w.version IS DISTINCT FROM EXCLUDED.version OR w.gone_at IS NOT NULL
				THEN now() ELSE w.version_since END,
			project_id = EXCLUDED.project_id, reason = EXCLUDED.reason, annotation = EXCLUDED.annotation,
			service = EXCLUDED.service, environment = EXCLUDED.environment, branch = EXCLUDED.branch,
			version = EXCLUDED.version, images = EXCLUDED.images, state = EXCLUDED.state,
			pending_version = EXCLUDED.pending_version, pending_since = EXCLUDED.pending_since,
			observed_at = now(), gone_at = NULL, updated_at = now()`, clusterID, string(batch))
	return err
}

func (s *WorkloadRepository) Unmatched(ctx context.Context, clusterID uuid.UUID, limit uint32, offset uint64) ([]deploytypes.Unmatched, uint64, error) {
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (deploytypes.Unmatched, error) {
		var u deploytypes.Unmatched
		var kind, reason string
		err := r.Scan(&u.Namespace, &kind, &u.Name, &reason, &u.Annotation, &u.ObservedAt)
		u.Kind, u.Reason = deploytypes.Kind(kind), deploytypes.Reason(reason)
		return u, err
	}, `
		SELECT w.namespace, w.kind, w.name, w.reason, w.annotation, rfc3339(w.observed_at) AS observed_at
		FROM cluster_workloads w
		WHERE w.cluster_id = $1
			AND w.project_id IS NULL
			AND w.gone_at IS NULL
		ORDER BY w.namespace, w.kind, w.name
		LIMIT $2
		OFFSET $3`, clusterID, int64(limit), int64(offset))
	if err != nil {
		return nil, 0, err
	}
	total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM cluster_workloads w
		WHERE w.cluster_id = $1
			AND w.project_id IS NULL
			AND w.gone_at IS NULL`, clusterID)
	if err != nil {
		return nil, 0, err
	}
	return items, uint64(max(total, 0)), nil
}

func (s *WorkloadRepository) ForProject(ctx context.Context, projectID uuid.UUID) ([]deploytypes.Observed, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (deploytypes.Observed, error) {
		var o deploytypes.Observed
		var kind string
		err := r.Scan(&o.Cluster, &o.Namespace, &kind, &o.Name, &o.Service, &o.Environment, &o.Branch, &o.Version,
			&o.Images, &o.State, &o.ObservedAt, &o.Gone)
		o.Kind = deploytypes.Kind(kind)
		return o, err
	}, `
		SELECT c.name AS cluster, w.namespace, w.kind, w.name, w.service, w.environment, w.branch,
			w.version, w.images, w.state, rfc3339(w.observed_at) AS observed_at, w.gone_at IS NOT NULL AS gone
		FROM cluster_workloads w
		JOIN clusters c ON c.id = w.cluster_id
		JOIN nodes n ON n.id = w.project_id
		WHERE w.project_id = $1
			AND n.cluster_observation
		ORDER BY w.service, w.environment, c.name, w.namespace, w.name`, projectID)
}

func (s *WorkloadRepository) ForPair(ctx context.Context, projectID uuid.UUID, service, environment string) ([]deploytypes.PairObservation, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (deploytypes.PairObservation, error) {
		var o deploytypes.PairObservation
		err := r.Scan(&o.Observed.Cluster, &o.Observed.Namespace, &o.Observed.Version, &o.VersionSince)
		o.Observed.Service, o.Observed.Environment = service, environment
		return o, err
	}, `
		SELECT c.name AS cluster, w.namespace, w.version, w.version_since
		FROM cluster_workloads w
		JOIN clusters c ON c.id = w.cluster_id
		JOIN nodes n ON n.id = w.project_id
		WHERE w.project_id = $1
			AND w.service = $2
			AND w.environment = $3
			AND w.gone_at IS NULL
			AND n.cluster_observation
		ORDER BY c.name, w.namespace, w.name`, projectID, service, environment)
}

func (s *WorkloadRepository) Prune(ctx context.Context, days uint32) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM cluster_workloads
		WHERE gone_at < now() - make_interval(days => $1::int)`, int(days))
}

func (s *WorkloadRepository) Project(ctx context.Context, ref string) (*deploytypes.ProjectRef, error) {
	if id, err := uuid.Parse(ref); err == nil {
		return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (deploytypes.ProjectRef, error) {
			var p deploytypes.ProjectRef
			err := r.Scan(&p.ID, &p.Observe)
			return p, err
		}, `
			SELECT id, cluster_observation
			FROM nodes
			WHERE id = $1
				AND kind = 'project'`, id)
	}
	var parent *uuid.UUID
	segments := strings.Split(ref, "/")
	for i, seg := range segments {
		var kind string
		n, err := postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (deploytypes.ProjectRef, error) {
			var p deploytypes.ProjectRef
			err := r.Scan(&p.ID, &kind, &p.Observe)
			return p, err
		}, `
			SELECT id, kind, cluster_observation
			FROM nodes
			WHERE parent_id IS NOT DISTINCT FROM $1
				AND slug = $2`, parent, strings.ToLower(seg))
		if n == nil || err != nil {
			return nil, err
		}
		if i == len(segments)-1 {
			if kind != "project" {
				return nil, nil
			}
			return n, nil
		}
		parent = &n.ID
	}
	return nil, nil
}

func (s *WorkloadRepository) Current(ctx context.Context, projectID uuid.UUID, service, environment string) (*string, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		SELECT d.version
		FROM service_environments e
		JOIN service_deployments d ON d.id = e.deployment_id
		WHERE e.project_id = $1
			AND e.service = $2
			AND e.environment = $3`, projectID, service, environment)
}
