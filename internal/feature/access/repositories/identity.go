package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type IdentityRepository struct{ db *pgtx.DB }

func NewIdentityRepository(db *pgtx.DB) *IdentityRepository { return &IdentityRepository{db: db} }

func (s *IdentityRepository) Find(ctx context.Context, provider, subject string) (*accesstypes.Identity, *accesstypes.User, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, scanIdentityUser, `
		SELECT i.id, i.user_id, i.provider, i.subject, i.email, rfc3339(i.created_at),
			rfc3339(i.last_login_at), u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at
		FROM user_identities i
		JOIN users u ON u.id = i.user_id
		WHERE i.provider = $1
			AND i.subject = $2`, provider, subject)
	if err != nil || found == nil {
		return nil, nil, err
	}
	return &found.identity, &found.user, nil
}

func (s *IdentityRepository) Insert(ctx context.Context, n accesstypes.NewIdentity) (accesstypes.Identity, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, scanIdentity, `
		INSERT INTO user_identities AS i (id, user_id, provider, subject, email)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING i.id, i.user_id, i.provider, i.subject, i.email, rfc3339(i.created_at),
			rfc3339(i.last_login_at)`,
		n.ID, n.UserID, n.Provider, n.Subject, n.Email)
}

func (s *IdentityRepository) Touch(ctx context.Context, id uuid.UUID, email *string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE user_identities
		SET last_login_at = now(), email = COALESCE($2, email)
		WHERE id = $1`, id, email)
	return err
}

func (s *IdentityRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]accesstypes.Identity, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanIdentity, `
		SELECT i.id, i.user_id, i.provider, i.subject, i.email, rfc3339(i.created_at),
			rfc3339(i.last_login_at)
		FROM user_identities i
		WHERE i.user_id = $1
		ORDER BY i.created_at, i.id`, userID)
}

func (s *IdentityRepository) CountForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM user_identities
		WHERE user_id = $1`, userID)
}

func (s *IdentityRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM user_identities
		WHERE id = $1
			AND user_id = $2`, id, userID)
}

type LoginStateRepository struct{ db *pgtx.DB }

func NewLoginStateRepository(db *pgtx.DB) *LoginStateRepository {
	return &LoginStateRepository{db: db}
}

func (s *LoginStateRepository) Insert(ctx context.Context, n accesstypes.NewLoginState) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO oauth_login_states (id, browser_hash, provider, state, nonce, code_verifier, next,
			link_user_id, expires_at, invite_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now() + make_interval(secs => $9), $10)`,
		n.ID, n.BrowserHash[:], n.Provider, n.State, n.Nonce, n.Verifier,
		n.Next, n.LinkUserID, float64(n.TTLSecs), n.InviteHash)
	return err
}

func (s *LoginStateRepository) Take(ctx context.Context, browserHash [32]byte, state, provider string) (*accesstypes.LoginState, error) {
	l, err := postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (accesstypes.LoginState, error) {
		var l accesstypes.LoginState
		err := r.Scan(&l.Provider, &l.Nonce, &l.Verifier, &l.Next, &l.LinkUserID, &l.InviteHash)
		return l, err
	}, `
		DELETE FROM oauth_login_states
		WHERE browser_hash = $1
			AND state = $2
			AND provider = $3
			AND expires_at > now()
		RETURNING provider, nonce, code_verifier, next, link_user_id, invite_hash`, browserHash[:], state, provider)
	return l, err
}

func (s *LoginStateRepository) DeleteByState(ctx context.Context, state string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM oauth_login_states
		WHERE state = $1`, state)
	return err
}

func (s *LoginStateRepository) Prune(ctx context.Context) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM oauth_login_states
		WHERE expires_at <= now()`)
}
