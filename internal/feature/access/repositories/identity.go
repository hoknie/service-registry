package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type IdentityRepository struct{ db *pgtx.DB }

func NewIdentityRepository(db *pgtx.DB) *IdentityRepository { return &IdentityRepository{db: db} }

func insertIdentity(ctx context.Context, db pgtx.Querier, n accesstypes.NewIdentity, userID uuid.UUID) (accesstypes.Identity, error) {
	return postgres.One(ctx, db, constraints, scanIdentity, `
		INSERT INTO user_identities AS i (id, user_id, provider, subject, email)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING i.id, i.user_id, i.provider, i.subject, i.email, rfc3339(i.created_at),
			rfc3339(i.last_login_at)`,
		n.ID, userID, n.Provider, n.Subject, n.Email)
}

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
	return insertIdentity(ctx, s.db.From(ctx), n, n.UserID)
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

func (s *IdentityRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		hasPassword, err := postgres.Find(ctx, tx, constraints, pgx.RowTo[bool], `
			SELECT password_hash IS NOT NULL
			FROM users
			WHERE id = $1
			FOR UPDATE`, userID)
		if err != nil {
			return err
		}
		if hasPassword == nil {
			return apperr.ErrNotFound
		}
		count, err := postgres.One(ctx, tx, constraints, pgx.RowTo[int64], `
			SELECT count(*)
			FROM user_identities
			WHERE user_id = $1`, userID)
		if err != nil {
			return err
		}
		if err := postgres.ExecFound(ctx, tx, constraints, `
			DELETE FROM user_identities
			WHERE id = $1
				AND user_id = $2`, id, userID); err != nil {
			return err
		}
		if !*hasPassword && count <= 1 {
			return accesstypes.ConflictLastLoginMethod
		}
		return nil
	})
	return postgres.Translate(err, constraints)
}

func (s *IdentityRepository) CreateUser(ctx context.Context, u accesstypes.NewUser, n accesstypes.NewIdentity) (accesstypes.User, error) {
	var user accesstypes.User
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var err error
		u.PasswordHash = ""
		if user, err = insertUser(ctx, tx, u); err != nil {
			return err
		}
		_, err = insertIdentity(ctx, tx, n, user.ID)
		return err
	})
	return user, postgres.Translate(err, constraints)
}

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
