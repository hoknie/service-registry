package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type LoginStateRepository struct{ db *pgtx.DB }

func NewLoginStateRepository(db *pgtx.DB) *LoginStateRepository {
	return &LoginStateRepository{db: db}
}

func (s *LoginStateRepository) Insert(ctx context.Context, n accesstypes.NewLoginState) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO oauth_login_states (id, browser_hash, provider, state, nonce, code_verifier, next,
			link_user_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() + make_interval(secs => $9))`,
		n.ID, n.BrowserHash[:], n.Provider, n.State, n.Nonce, n.Verifier,
		n.Next, n.LinkUserID, float64(n.TTLSecs))
	return err
}

func (s *LoginStateRepository) Take(ctx context.Context, browserHash [32]byte, state, provider string) (*accesstypes.LoginState, error) {
	l, err := postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (accesstypes.LoginState, error) {
		var l accesstypes.LoginState
		err := r.Scan(&l.Provider, &l.Nonce, &l.Verifier, &l.Next, &l.LinkUserID)
		return l, err
	}, `
		DELETE FROM oauth_login_states
		WHERE browser_hash = $1
			AND state = $2
			AND provider = $3
			AND expires_at > now()
		RETURNING provider, nonce, code_verifier, next, link_user_id`, browserHash[:], state, provider)
	if err != nil || l != nil {
		return l, err
	}
	if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM oauth_login_states
		WHERE state = $1`, state); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *LoginStateRepository) Prune(ctx context.Context) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM oauth_login_states
		WHERE expires_at <= now()`)
}
