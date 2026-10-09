package ingest

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/access"
	domain "svc-registry/internal/ingest"
	"svc-registry/internal/postgres"
)

type EventStore struct{ pool *pgxpool.Pool }

func NewEventStore(pool *pgxpool.Pool) *EventStore { return &EventStore{pool: pool} }

const pruneBatch = 5000

func loadEvent(ctx context.Context, tx pgx.Tx, id uuid.UUID) (domain.Event, error) {
	rows, _ := tx.Query(ctx, `
		SELECT e.id, e.project_id, e.type, e.version, e.idempotency_key,
			rfc3339(e.occurred_at) AS occurred_at, rfc3339(e.received_at) AS received_at,
			k.prefix AS key_prefix, e.payload
		FROM project_events e
		LEFT JOIN project_keys k ON k.id = e.key_id
		WHERE e.id = $1`, id)
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[eventRow])
	if err != nil {
		return domain.Event{}, dbErr(err)
	}
	return r.event(), nil
}

func loadResult(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (*domain.DeploymentResult, error) {
	rows, _ := tx.Query(ctx, `
		SELECT id, service, environment, version, became_current
		FROM service_deployments
		WHERE event_id = $1`, eventID)
	d, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[domain.DeploymentResult])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &d, nil
}

func applyDeployed(ctx context.Context, tx pgx.Tx, e domain.NewEvent, p *domain.ServiceDeployed) (domain.DeploymentResult, error) {
	at := e.Envelope.OccurredAt
	eventID := e.ID
	became, err := postgres.InsertDeployment(ctx, tx, postgres.Deployment{
		ID: e.DeploymentID, EnvironmentID: e.EnvironmentID, ProjectID: e.ProjectID, EventID: &eventID, Source: "event",
		Service: p.Service, Environment: p.Environment, Version: p.Version, CommitSHA: p.CommitSHA, Branch: p.Branch,
		Cluster: p.Cluster, Namespace: p.Namespace, URL: p.URL, DeployedBy: p.DeployedBy, Metadata: p.Metadata, OccurredAt: at,
	})
	if err != nil {
		return domain.DeploymentResult{}, dbErr(err)
	}
	if p.Branch != nil {
		if name := strings.TrimPrefix(*p.Branch, "refs/heads/"); name != "" {
			if err := postgres.UpsertIngestBranch(ctx, tx, e.ProjectID, name, p.CommitSHA, at); err != nil {
				return domain.DeploymentResult{}, dbErr(err)
			}
		}
	}
	return domain.DeploymentResult{
		ID: e.DeploymentID, Service: p.Service, Environment: p.Environment, Version: p.Version, BecameCurrent: became,
	}, nil
}

func (s *EventStore) Accept(ctx context.Context, e domain.NewEvent) (domain.Accepted, error) {
	env := e.Envelope
	payload := string(env.Payload)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Accepted{}, dbErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var future bool
	if err := tx.QueryRow(ctx, `
		SELECT $1::timestamptz > now() + interval '5 minutes'`, env.OccurredAt).Scan(&future); err != nil {
		return domain.Accepted{}, dbErr(err)
	}
	if future {
		return domain.Accepted{}, domain.Invalid(domain.FieldError{Path: "occurred_at", Code: domain.FieldInFuture})
	}

	var inserted uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO project_events (id, project_id, key_id, type, version, idempotency_key, occurred_at,
			payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7::timestamptz, $8::jsonb)
		ON CONFLICT (project_id, idempotency_key)
		DO NOTHING
		RETURNING id`, e.ID, e.ProjectID, e.KeyID, string(env.Type), env.Version,
		env.IdempotencyKey, env.OccurredAt, payload).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		var id uuid.UUID
		var same bool
		err := tx.QueryRow(ctx, `
			SELECT id,
				type = $3 AND version = $4 AND occurred_at = $5::timestamptz AND payload = $6::jsonb AS same
			FROM project_events
			WHERE project_id = $1
				AND idempotency_key = $2`, e.ProjectID, env.IdempotencyKey, string(env.Type), env.Version,
			env.OccurredAt, payload).Scan(&id, &same)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Accepted{}, &domain.InternalError{Detail: "idempotent event vanished during replay"}
		}
		if err != nil {
			return domain.Accepted{}, dbErr(err)
		}
		if !same {
			return domain.Accepted{}, domain.ConflictIdempotencyKeyReused
		}
		event, err := loadEvent(ctx, tx, id)
		if err != nil {
			return domain.Accepted{}, err
		}
		deployment, err := loadResult(ctx, tx, id)
		if err != nil {
			return domain.Accepted{}, err
		}
		return domain.Accepted{Event: event, Replayed: true, Deployment: deployment}, nil
	}
	if err != nil {
		return domain.Accepted{}, dbErr(err)
	}

	var deployment *domain.DeploymentResult
	if env.ServiceDeployed != nil {
		d, err := applyDeployed(ctx, tx, e, env.ServiceDeployed)
		if err != nil {
			return domain.Accepted{}, err
		}
		deployment = &d
	}
	event, err := loadEvent(ctx, tx, e.ID)
	if err != nil {
		return domain.Accepted{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Accepted{}, dbErr(err)
	}
	return domain.Accepted{Event: event, Deployment: deployment}, nil
}

func (s *EventStore) List(ctx context.Context, projectID uuid.UUID, f domain.EventFilter, page access.PageRequest) (access.Page[domain.Event], error) {
	var total int64
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM project_events e
		WHERE e.project_id = $1
			AND ($2::text IS NULL OR e.type = $2)`, projectID, f.Type).Scan(&total); err != nil {
		return access.Page[domain.Event]{}, dbErr(err)
	}
	rows, _ := s.pool.Query(ctx, `
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
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Event, error) {
		row, err := pgx.RowToStructByName[eventRow](r)
		return row.event(), err
	})
	if err != nil {
		return access.Page[domain.Event]{}, dbErr(err)
	}
	return access.Page[domain.Event]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *EventStore) Prune(ctx context.Context, retentionDays uint32) (uint64, error) {
	days := int32(min(retentionDays, uint32(1<<31-1)))
	var deleted uint64
	for {
		tag, err := s.pool.Exec(ctx, `
			DELETE FROM project_events
			WHERE id IN (
				SELECT id
				FROM project_events
				WHERE received_at < now() - make_interval(days => $1)
				LIMIT $2
			)`, days, int64(pruneBatch))
		if err != nil {
			return deleted, dbErr(err)
		}
		n := tag.RowsAffected()
		deleted += uint64(n)
		if n < pruneBatch {
			return deleted, nil
		}
	}
}
