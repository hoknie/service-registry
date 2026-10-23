package repositories

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type EpisodeRepository struct{ db *pgtx.DB }

func NewEpisodeRepository(db *pgtx.DB) *EpisodeRepository { return &EpisodeRepository{db: db} }

func (s *EpisodeRepository) Open(ctx context.Context, ep notifytypes.NewEpisode, eventID uuid.UUID) (bool, error) {
	key, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		INSERT INTO notification_episodes (key, id, type, node_id, signature, event_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (key)
		DO UPDATE SET signature = EXCLUDED.signature, event_id = EXCLUDED.event_id, updated_at = now()
		WHERE notification_episodes.signature <> EXCLUDED.signature
		RETURNING key`, ep.Key, uuid.Must(uuid.NewV7()), string(ep.Type), ep.NodeID, ep.Signature, eventID)
	return key != nil, err
}

func (s *EpisodeRepository) Close(ctx context.Context, key string) (*uuid.UUID, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		DELETE FROM notification_episodes
		WHERE key = $1
		RETURNING event_id`, key)
}

func (s *EpisodeRepository) Recipients(ctx context.Context, eventID uuid.UUID) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT subscription_id
		FROM notification_deliveries
		WHERE event_id = $1`, eventID)
}

func (s *EpisodeRepository) OpenOf(ctx context.Context, typ notifytypes.EventType) ([]notifytypes.Episode, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (notifytypes.Episode, error) {
		var ep notifytypes.Episode
		var data []byte
		err := r.Scan(&ep.Key, &ep.NodeID, &data)
		ep.Data = json.RawMessage(data)
		return ep, err
	}, `
		SELECT ep.key, ep.node_id, e.payload -> 'data'
		FROM notification_episodes ep
		JOIN notification_events e ON e.id = ep.event_id
		WHERE ep.type = $1
		ORDER BY ep.opened_at, ep.key`, string(typ))
}
