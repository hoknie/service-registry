package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type EventRepository struct{ db *pgtx.DB }

func NewEventRepository(db *pgtx.DB) *EventRepository { return &EventRepository{db: db} }

const pruneBatch = 5000

func (s *EventRepository) Insert(ctx context.Context, e ingesttypes.NewEvent) (bool, error) {
	env := e.Envelope
	inserted, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		INSERT INTO project_events (id, project_id, key_id, type, version, idempotency_key, occurred_at,
			payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7::timestamptz, $8::jsonb)
		ON CONFLICT (project_id, idempotency_key)
		DO NOTHING
		RETURNING id`, e.ID, e.ProjectID, e.KeyID, string(env.Type), env.Version,
		env.IdempotencyKey, env.OccurredAt, string(env.Payload))
	return inserted != nil, err
}

func (s *EventRepository) FindReplay(ctx context.Context, e ingesttypes.NewEvent) (*ingesttypes.Replay, error) {
	env := e.Envelope
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(replayRow.replay), `
		SELECT id,
			type = $3 AND version = $4 AND occurred_at = $5::timestamptz AND payload = $6::jsonb AS same
		FROM project_events
		WHERE project_id = $1
			AND idempotency_key = $2`, e.ProjectID, env.IdempotencyKey, string(env.Type), env.Version,
		env.OccurredAt, string(env.Payload))
}

func (s *EventRepository) Load(ctx context.Context, id uuid.UUID) (ingesttypes.Event, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, postgres.Struct(eventRow.event), `
		SELECT e.id, e.project_id, e.type, e.version, e.idempotency_key,
			rfc3339(e.occurred_at) AS occurred_at, rfc3339(e.received_at) AS received_at,
			k.prefix AS key_prefix, e.payload
		FROM project_events e
		LEFT JOIN project_keys k ON k.id = e.key_id
		WHERE e.id = $1`, id)
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
