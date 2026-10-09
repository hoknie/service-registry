package access

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/access"
)

type LoginStateStore struct{ pool *pgxpool.Pool }

func NewLoginStateStore(pool *pgxpool.Pool) *LoginStateStore { return &LoginStateStore{pool: pool} }

const (
	loginStateInsert = "INSERT INTO oauth_login_states (id, browser_hash, provider, state, nonce, code_verifier, next, " +
		"link_user_id, expires_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() + make_interval(secs => $9))"
	loginStateTake = "DELETE FROM oauth_login_states WHERE browser_hash = $1 AND state = $2 AND provider = $3 " +
		"AND expires_at > now() RETURNING provider, nonce, code_verifier, next, link_user_id"
	loginStateConsume = "DELETE FROM oauth_login_states WHERE state = $1"
	loginStatePrune   = "DELETE FROM oauth_login_states WHERE expires_at <= now()"
)

func (s *LoginStateStore) Insert(ctx context.Context, n domain.NewLoginState) error {
	_, err := s.pool.Exec(ctx, loginStateInsert, n.ID, n.BrowserHash[:], n.Provider, n.State, n.Nonce, n.Verifier,
		n.Next, n.LinkUserID, float64(n.TTLSecs))
	return dbErr(err)
}

func (s *LoginStateStore) Take(ctx context.Context, browserHash [32]byte, state, provider string) (*domain.LoginState, error) {
	var l domain.LoginState
	err := s.pool.QueryRow(ctx, loginStateTake, browserHash[:], state, provider).
		Scan(&l.Provider, &l.Nonce, &l.Verifier, &l.Next, &l.LinkUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := s.pool.Exec(ctx, loginStateConsume, state); err != nil {
			return nil, dbErr(err)
		}
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &l, nil
}

func (s *LoginStateStore) Prune(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, loginStatePrune)
	if err != nil {
		return 0, dbErr(err)
	}
	return tag.RowsAffected(), nil
}
