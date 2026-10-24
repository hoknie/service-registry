package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type AgentTokenRepository struct{ db *pgtx.DB }

func NewAgentTokenRepository(db *pgtx.DB) *AgentTokenRepository { return &AgentTokenRepository{db: db} }

func (s *AgentTokenRepository) ExpireActive(ctx context.Context, clusterID uuid.UUID, graceSecs uint64) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE cluster_agent_tokens
		SET expires_at = LEAST(COALESCE(expires_at, 'infinity'), now() + make_interval(secs => $2))
		WHERE cluster_id = $1
			AND revoked_at IS NULL
			AND (expires_at IS NULL OR expires_at > now())`, clusterID, float64(min(graceSecs, 1<<53)))
	return err
}

func (s *AgentTokenRepository) Insert(ctx context.Context, t deploytypes.NewAgentToken) (deploytypes.AgentToken, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[deploytypes.AgentToken], `
		INSERT INTO cluster_agent_tokens AS t (id, cluster_id, prefix, token_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING t.id, t.prefix, rfc3339(t.created_at) AS created_at, rfc3339(t.expires_at) AS expires_at`,
		t.ID, t.ClusterID, t.Prefix, t.Hash[:])
}

func (s *AgentTokenRepository) RevokeAll(ctx context.Context, clusterID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE cluster_agent_tokens
		SET revoked_at = now()
		WHERE cluster_id = $1
			AND revoked_at IS NULL`, clusterID)
	return err
}

func (s *AgentTokenRepository) FindActive(ctx context.Context, clusterID uuid.UUID, hash [32]byte) (*deploytypes.AgentTokenUse, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[deploytypes.AgentTokenUse], `
		SELECT t.id, t.last_used_at IS NULL OR t.last_used_at < now() - interval '60 seconds' AS stale
		FROM cluster_agent_tokens t
		JOIN clusters c ON c.id = t.cluster_id
			AND c.mode = 'agent'
		WHERE t.token_hash = $1
			AND t.cluster_id = $2
			AND t.revoked_at IS NULL
			AND (t.expires_at IS NULL OR t.expires_at > now())`, hash[:], clusterID)
}

func (s *AgentTokenRepository) Touch(ctx context.Context, id uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE cluster_agent_tokens
		SET last_used_at = now()
		WHERE id = $1`, id)
	return err
}
