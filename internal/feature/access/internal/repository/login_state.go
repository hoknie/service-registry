package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/postgres"
)

type LoginStates struct{ db *postgres.DB }

func NewLoginStates(db *postgres.DB) *LoginStates {
	return &LoginStates{db: db}
}

func (s *LoginStates) Insert(ctx context.Context, n access.NewLoginState) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO oauth_login_states (id, browser_hash, provider, state, nonce, code_verifier, next,
			link_user_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() + make_interval(secs => $9))`,
		n.ID, n.BrowserHash[:], n.Provider, n.State, n.Nonce, n.Verifier,
		n.Next, n.LinkUserID, float64(n.TTLSecs))
	return dbErr(err)
}

func (s *LoginStates) Take(ctx context.Context, browserHash [32]byte, state, provider string) (*access.LoginState, error) {
	var l access.LoginState
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
			return nil, dbErr(err)
		}
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &l, nil
}

func (s *LoginStates) Prune(ctx context.Context) (int64, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM oauth_login_states
		WHERE expires_at <= now()`)
	if err != nil {
		return 0, dbErr(err)
	}
	return tag.RowsAffected(), nil
}
