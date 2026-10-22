package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
)

type LoginStateRepository struct{ db *postgres.DB }

func NewLoginStateRepository(db *postgres.DB) *LoginStateRepository {
	return &LoginStateRepository{db: db}
}

func (s *LoginStateRepository) Insert(ctx context.Context, n accesstypes.NewLoginState) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO oauth_login_states (id, browser_hash, provider, state, nonce, code_verifier, next,
			link_user_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() + make_interval(secs => $9))`,
		n.ID, n.BrowserHash[:], n.Provider, n.State, n.Nonce, n.Verifier,
		n.Next, n.LinkUserID, float64(n.TTLSecs))
	return postgres.Translate(err, constraints)
}

func (s *LoginStateRepository) Take(ctx context.Context, browserHash [32]byte, state, provider string) (*accesstypes.LoginState, error) {
	var l accesstypes.LoginState
	err := s.db.From(ctx).QueryRow(ctx, `
		DELETE FROM oauth_login_states
		WHERE browser_hash = $1
			AND state = $2
			AND provider = $3
			AND expires_at > now()
		RETURNING provider, nonce, code_verifier, next, link_user_id`, browserHash[:], state, provider).
		Scan(&l.Provider, &l.Nonce, &l.Verifier, &l.Next, &l.LinkUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := s.db.From(ctx).Exec(ctx, `
			DELETE FROM oauth_login_states
			WHERE state = $1`, state); err != nil {
			return nil, postgres.Translate(err, constraints)
		}
		return nil, nil
	}
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	return &l, nil
}

func (s *LoginStateRepository) Prune(ctx context.Context) (int64, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM oauth_login_states
		WHERE expires_at <= now()`)
	if err != nil {
		return 0, postgres.Translate(err, constraints)
	}
	return tag.RowsAffected(), nil
}
