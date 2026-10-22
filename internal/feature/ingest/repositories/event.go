package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type EventRepository struct{ db *pgtx.DB }

func NewEventRepository(db *pgtx.DB) *EventRepository { return &EventRepository{db: db} }

const pruneBatch = 5000

func loadEvent(ctx context.Context, tx pgtx.Querier, id uuid.UUID) (ingesttypes.Event, error) {
	return postgres.One(ctx, tx, constraints, postgres.Struct(eventRow.event), `
		SELECT e.id, e.project_id, e.type, e.version, e.idempotency_key,
			rfc3339(e.occurred_at) AS occurred_at, rfc3339(e.received_at) AS received_at,
			k.prefix AS key_prefix, e.payload
		FROM project_events e
		LEFT JOIN project_keys k ON k.id = e.key_id
		WHERE e.id = $1`, id)
}

func loadResult(ctx context.Context, tx pgtx.Querier, eventID uuid.UUID) (*ingesttypes.DeploymentResult, error) {
	return postgres.Find(ctx, tx, constraints, pgx.RowToStructByPos[ingesttypes.DeploymentResult], `
		SELECT id, service, environment, version, became_current
		FROM service_deployments
		WHERE event_id = $1`, eventID)
}

func applyDeployed(ctx context.Context, tx pgtx.Querier, e ingesttypes.NewEvent, p *ingesttypes.ServiceDeployed) (ingesttypes.DeploymentResult, error) {
	at := e.Envelope.OccurredAt
	became, err := InsertDeployment(ctx, tx, ingesttypes.DeploymentRecord{
		ID: e.DeploymentID, EnvironmentID: e.EnvironmentID, ProjectID: e.ProjectID, EventID: new(e.ID), Source: "event",
		Service: p.Service, Environment: p.Environment, Version: p.Version, CommitSHA: p.CommitSHA, Branch: p.Branch,
		Cluster: p.Cluster, Namespace: p.Namespace, URL: p.URL, DeployedBy: p.DeployedBy, Metadata: p.Metadata, Links: p.Links, OccurredAt: at,
	})
	if err != nil {
		return ingesttypes.DeploymentResult{}, err
	}
	return ingesttypes.DeploymentResult{
		ID: e.DeploymentID, Service: p.Service, Environment: p.Environment, Version: p.Version, BecameCurrent: became,
	}, nil
}

func (s *EventRepository) Accept(ctx context.Context, e ingesttypes.NewEvent) (ingesttypes.Accepted, error) {
	env := e.Envelope
	payload := string(env.Payload)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return ingesttypes.Accepted{}, postgres.Translate(err, constraints)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	future, err := postgres.One(ctx, tx, constraints, pgx.RowTo[bool], `
		SELECT $1::timestamptz > now() + interval '5 minutes'`, env.OccurredAt)
	if err != nil {
		return ingesttypes.Accepted{}, err
	}
	if future {
		return ingesttypes.Accepted{}, ingesttypes.Invalid(ingesttypes.FieldError{Path: "occurred_at", Code: ingesttypes.FieldInFuture})
	}

	inserted, err := postgres.Find(ctx, tx, constraints, pgx.RowTo[uuid.UUID], `
		INSERT INTO project_events (id, project_id, key_id, type, version, idempotency_key, occurred_at,
			payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7::timestamptz, $8::jsonb)
		ON CONFLICT (project_id, idempotency_key)
		DO NOTHING
		RETURNING id`, e.ID, e.ProjectID, e.KeyID, string(env.Type), env.Version,
		env.IdempotencyKey, env.OccurredAt, payload)
	if err != nil {
		return ingesttypes.Accepted{}, err
	}
	if inserted == nil {
		replay, err := postgres.Find(ctx, tx, constraints, pgx.RowToStructByName[replayRow], `
			SELECT id,
				type = $3 AND version = $4 AND occurred_at = $5::timestamptz AND payload = $6::jsonb AS same
			FROM project_events
			WHERE project_id = $1
				AND idempotency_key = $2`, e.ProjectID, env.IdempotencyKey, string(env.Type), env.Version,
			env.OccurredAt, payload)
		if err != nil {
			return ingesttypes.Accepted{}, err
		}
		if replay == nil {
			return ingesttypes.Accepted{}, apperr.InternalDetail("idempotent event vanished during replay")
		}
		if !replay.Same {
			return ingesttypes.Accepted{}, ingesttypes.ConflictIdempotencyKeyReused
		}
		event, err := loadEvent(ctx, tx, replay.ID)
		if err != nil {
			return ingesttypes.Accepted{}, err
		}
		deployment, err := loadResult(ctx, tx, replay.ID)
		if err != nil {
			return ingesttypes.Accepted{}, err
		}
		return ingesttypes.Accepted{Event: event, Replayed: true, Deployment: deployment}, nil
	}

	var deployment *ingesttypes.DeploymentResult
	if env.ServiceDeployed != nil {
		d, err := applyDeployed(ctx, tx, e, env.ServiceDeployed)
		if err != nil {
			return ingesttypes.Accepted{}, err
		}
		deployment = &d
	}
	event, err := loadEvent(ctx, tx, e.ID)
	if err != nil {
		return ingesttypes.Accepted{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ingesttypes.Accepted{}, postgres.Translate(err, constraints)
	}
	return ingesttypes.Accepted{Event: event, Deployment: deployment}, nil
}

func (s *EventRepository) List(ctx context.Context, projectID uuid.UUID, f ingesttypes.EventFilter, page accesstypes.PageRequest) (accesstypes.Page[ingesttypes.Event], error) {
	total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM project_events e
		WHERE e.project_id = $1
			AND ($2::text IS NULL OR e.type = $2)`, projectID, f.Type)
	if err != nil {
		return accesstypes.Page[ingesttypes.Event]{}, err
	}
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(eventRow.event), `
		SELECT e.id, e.project_id, e.type, e.version, e.idempotency_key,
			rfc3339(e.occurred_at) AS occurred_at, rfc3339(e.received_at) AS received_at,
			k.prefix AS key_prefix, e.payload
		FROM project_events e
		LEFT JOIN project_keys k ON k.id = e.key_id
		WHERE e.project_id = $1
			AND ($2::text IS NULL OR e.type = $2)
		ORDER BY e.received_at DESC, e.id DESC
		LIMIT $3
		OFFSET $4`, projectID, f.Type, int64(page.Limit), bigint(page.Offset))
	if err != nil {
		return accesstypes.Page[ingesttypes.Event]{}, err
	}
	return accesstypes.Page[ingesttypes.Event]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *EventRepository) Prune(ctx context.Context, retentionDays uint32) (uint64, error) {
	days := int32(min(retentionDays, uint32(1<<31-1)))
	var deleted uint64
	for {
		n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
			DELETE FROM project_events
			WHERE id IN (
				SELECT id
				FROM project_events
				WHERE received_at < now() - make_interval(days => $1)
				LIMIT $2
			)`, days, int64(pruneBatch))
		if err != nil {
			return deleted, err
		}
		deleted += uint64(n)
		if n < pruneBatch {
			return deleted, nil
		}
	}
}
