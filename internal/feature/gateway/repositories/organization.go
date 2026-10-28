package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type OrganizationRepository struct{ db *pgtx.DB }

func NewOrganizationRepository(db *pgtx.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

type OrganizationRow struct {
	Description string `db:"description"`
	Mode        string `db:"set_mode"`
}

func (s *OrganizationRepository) Get(ctx context.Context, org uuid.UUID) (OrganizationRow, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(func(r OrganizationRow) OrganizationRow { return r }), `
		SELECT description, set_mode
		FROM mcp_organizations
		WHERE organization_id = $1`, org)
	if err != nil || found == nil {
		return OrganizationRow{Mode: gatewaytypes.SetAll}, err
	}
	return *found, nil
}

func (s *OrganizationRepository) Modes(ctx context.Context, orgs []uuid.UUID) (map[uuid.UUID]string, error) {
	return postgres.Pairs[uuid.UUID, string](ctx, s.db.From(ctx), constraints, `
		SELECT organization_id, set_mode
		FROM mcp_organizations
		WHERE organization_id = ANY($1)`, orgs)
}

func (s *OrganizationRepository) SetDescription(ctx context.Context, org uuid.UUID, description string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO mcp_organizations (organization_id, description)
		VALUES ($1, $2)
		ON CONFLICT (organization_id)
		DO UPDATE SET description = $2, updated_at = now()`, org, description)
	return err
}

func (s *OrganizationRepository) Servers(ctx context.Context, org uuid.UUID) ([]gatewaytypes.ServerRef, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, serverRef, `
		SELECT s.id, s.slug, s.title, s.organization_id, s.status
		FROM mcp_organization_servers os
		JOIN mcp_servers s ON s.id = os.server_id
		WHERE os.organization_id = $1
		ORDER BY lower(s.title), s.id`, org)
}

func (s *OrganizationRepository) Memberships(ctx context.Context, orgs []uuid.UUID) ([]Membership, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(func(r Membership) Membership { return r }), `
		SELECT organization_id, server_id
		FROM mcp_organization_servers
		WHERE organization_id = ANY($1)`, orgs)
}

type Membership struct {
	OrganizationID uuid.UUID `db:"organization_id"`
	ServerID       uuid.UUID `db:"server_id"`
}

func (s *OrganizationRepository) Fit(ctx context.Context, org uuid.UUID, servers []uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT count(*) = cardinality($2::uuid[])
		FROM mcp_servers
		WHERE id = ANY($2)
			AND (organization_id IS NULL OR organization_id = $1)`, org, servers)
}

func (s *OrganizationRepository) ReplaceSet(ctx context.Context, org uuid.UUID, mode string, servers []uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH saved AS (
			INSERT INTO mcp_organizations (organization_id, set_mode)
			VALUES ($1, $2)
			ON CONFLICT (organization_id)
			DO UPDATE SET set_mode = $2, updated_at = now()
		),
		gone AS (
			DELETE FROM mcp_organization_servers
			WHERE organization_id = $1
				AND server_id <> ALL($3)
		)
		INSERT INTO mcp_organization_servers (organization_id, server_id)
		SELECT $1::uuid, s
		FROM unnest($3::uuid[]) AS s
		ON CONFLICT DO NOTHING`, org, mode, servers)
	return err
}

func serverRef(r pgx.CollectableRow) (gatewaytypes.ServerRef, error) {
	var ref gatewaytypes.ServerRef
	var status string
	err := r.Scan(&ref.ID, &ref.Slug, &ref.Title, &ref.OrganizationID, &status)
	ref.Status = gatewaytypes.Status(status)
	return ref, err
}

func (s *OrganizationRepository) Refs(ctx context.Context, ids []uuid.UUID) ([]gatewaytypes.ServerRef, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, serverRef, `
		SELECT s.id, s.slug, s.title, s.organization_id, s.status
		FROM unnest($1::uuid[]) WITH ORDINALITY AS i(id, n)
		JOIN mcp_servers s ON s.id = i.id
		ORDER BY i.n`, ids)
}
