package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type InvitationRepository struct{ db *pgtx.DB }

func NewInvitationRepository(db *pgtx.DB) *InvitationRepository { return &InvitationRepository{db: db} }

type invitationFilter struct {
	org  *uuid.UUID
	id   *uuid.UUID
	hash []byte
	open bool
}

func invitations(ctx context.Context, q pgtx.Querier, f invitationFilter) ([]membershiptypes.Invitation, error) {
	return postgres.List(ctx, q, constraints, postgres.Struct(invitationRow.invitation), `
		SELECT i.id, i.organization_id, o.name AS organization, i.email, i.role, i.locale,
			a.id AS invited_by_id, a.display_name AS invited_by_name, rfc3339(i.created_at) AS created_at,
			rfc3339(i.expires_at) AS expires_at, rfc3339(i.accepted_at) AS accepted_at, rfc3339(i.revoked_at) AS revoked_at,
			i.expires_at <= now() AS expired
		FROM organization_invitations i
		JOIN nodes o ON o.id = i.organization_id
		LEFT JOIN users a ON a.id = i.invited_by
		WHERE ($1::uuid IS NULL OR i.organization_id = $1)
			AND ($2::uuid IS NULL OR i.id = $2)
			AND ($3::bytea IS NULL OR i.token_hash = $3)
			AND (NOT $4 OR (i.accepted_at IS NULL AND i.revoked_at IS NULL))
		ORDER BY i.created_at DESC, i.id`, f.org, f.id, f.hash, f.open)
}

func first(list []membershiptypes.Invitation, err error) (*membershiptypes.Invitation, error) {
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

func (s *InvitationRepository) Insert(ctx context.Context, n membershiptypes.NewInvitation) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO organization_invitations (id, organization_id, email, role, locale, token_hash, invited_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, n.ID, n.OrganizationID, n.Email, string(n.Role), n.Locale, n.TokenHash[:],
		n.InvitedBy, n.ExpiresAt)
	return err
}

func (s *InvitationRepository) Waiting(ctx context.Context, org uuid.UUID) (int64, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM organization_invitations
		WHERE organization_id = $1
			AND accepted_at IS NULL
			AND revoked_at IS NULL
			AND expires_at > now()`, org)
}

func (s *InvitationRepository) DropExpired(ctx context.Context, org uuid.UUID, email string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE organization_invitations
		SET revoked_at = now()
		WHERE organization_id = $1
			AND email = $2
			AND accepted_at IS NULL
			AND revoked_at IS NULL
			AND expires_at <= now()`, org, email)
	return err
}

func (s *InvitationRepository) List(ctx context.Context, org uuid.UUID) ([]membershiptypes.Invitation, error) {
	return invitations(ctx, s.db.From(ctx), invitationFilter{org: &org, open: true})
}

func (s *InvitationRepository) Get(ctx context.Context, org, id uuid.UUID) (*membershiptypes.Invitation, error) {
	return first(invitations(ctx, s.db.From(ctx), invitationFilter{org: &org, id: &id}))
}

func (s *InvitationRepository) ByHash(ctx context.Context, hash [32]byte) (*membershiptypes.Invitation, error) {
	return first(invitations(ctx, s.db.From(ctx), invitationFilter{hash: hash[:]}))
}

func (s *InvitationRepository) Lock(ctx context.Context, hash [32]byte) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT id
		FROM organization_invitations
		WHERE token_hash = $1
		FOR UPDATE`, hash[:])
	return found != nil, err
}

func (s *InvitationRepository) Rotate(ctx context.Context, id uuid.UUID, hash [32]byte, locale string, expires time.Time) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE organization_invitations
		SET token_hash = $2, locale = $3, expires_at = $4, created_at = now()
		WHERE id = $1`, id, hash[:], locale, expires)
}

func (s *InvitationRepository) Revoke(ctx context.Context, org, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE organization_invitations
		SET revoked_at = now()
		WHERE organization_id = $1
			AND id = $2
			AND accepted_at IS NULL
			AND revoked_at IS NULL`, org, id)
}

func (s *InvitationRepository) Accept(ctx context.Context, id, user uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE organization_invitations
		SET accepted_at = now(), accepted_by = $2
		WHERE id = $1
			AND accepted_at IS NULL
			AND revoked_at IS NULL`, id, user)
}

func (s *InvitationRepository) Prune(ctx context.Context) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM organization_invitations
		WHERE COALESCE(accepted_at, revoked_at, expires_at) < now() - interval '30 days'`)
}
