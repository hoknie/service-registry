package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type MemberRepository struct{ db *pgtx.DB }

func NewMemberRepository(db *pgtx.DB) *MemberRepository { return &MemberRepository{db: db} }

type MemberFilter struct {
	Organization uuid.UUID
	Pattern      *string
	AfterName    *string
	AfterID      *uuid.UUID
	Limit        int
	UserID       *uuid.UUID
}

func (s *MemberRepository) List(ctx context.Context, f MemberFilter) ([]membershiptypes.Member, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(memberRow.member), `
		SELECT u.id, u.display_name AS name, u.email, u.is_service AS service, m.source,
			a.id AS added_by_id, a.display_name AS added_by_name, rfc3339(m.created_at) AS added_at,
			lower(u.display_name) || chr(1) || u.id::text AS cursor
		FROM organization_members m
		JOIN users u ON u.id = m.user_id
		LEFT JOIN users a ON a.id = m.added_by
		WHERE m.organization_id = $1
			AND ($2::text IS NULL OR lower(u.display_name) LIKE $2 OR lower(u.email) LIKE $2)
			AND ($3::text IS NULL OR (lower(u.display_name), u.id) > ($3, $4::uuid))
			AND ($6::uuid IS NULL OR u.id = $6)
		ORDER BY lower(u.display_name), u.id
		LIMIT $5`, f.Organization, f.Pattern, f.AfterName, f.AfterID, f.Limit, f.UserID)
}

func (s *MemberRepository) Candidates(ctx context.Context, org uuid.UUID, pattern string, limit int) ([]membershiptypes.Person, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(personRow.person), `
		SELECT u.id, u.display_name AS name, u.email, u.is_service AS service
		FROM users u
		WHERE u.status = 'active'
			AND (lower(u.display_name) LIKE $2 OR lower(u.email) LIKE $2)
			AND NOT EXISTS (SELECT 1 FROM organization_members m WHERE m.organization_id = $1 AND m.user_id = u.id)
		ORDER BY lower(u.display_name), u.id
		LIMIT $3`, org, pattern, limit)
}

func (s *MemberRepository) UserByEmail(ctx context.Context, email string) (*membershiptypes.Person, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(personRow.person), `
		SELECT u.id, u.display_name AS name, u.email, u.is_service AS service
		FROM users u
		WHERE lower(u.email) = $1`, email)
}

func (s *MemberRepository) User(ctx context.Context, id uuid.UUID) (*membershiptypes.Person, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(personRow.person), `
		SELECT u.id, u.display_name AS name, u.email, u.is_service AS service
		FROM users u
		WHERE u.id = $1
			AND u.status = 'active'`, id)
}

func (s *MemberRepository) IsMember(ctx context.Context, org, user uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT EXISTS (SELECT 1 FROM organization_members WHERE organization_id = $1 AND user_id = $2)`, org, user)
}

func (s *MemberRepository) Add(ctx context.Context, org, user uuid.UUID, source membershiptypes.Source, addedBy *uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		WITH ins AS (
			INSERT INTO organization_members (organization_id, user_id, source, added_by)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (organization_id, user_id) DO NOTHING
			RETURNING 1
		)
		SELECT EXISTS (SELECT 1 FROM ins)`, org, user, string(source), addedBy)
}

func (s *MemberRepository) Remove(ctx context.Context, org, user uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT remove_organization_member($1, $2)`, org, user)
}

func (s *MemberRepository) Admins(ctx context.Context, org uuid.UUID) (int64, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT organization_admins($1)`, org)
}
