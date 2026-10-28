package repositories

import (
	"context"

	"github.com/google/uuid"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ServerRepository struct{ db *pgtx.DB }

func NewServerRepository(db *pgtx.DB) *ServerRepository { return &ServerRepository{db: db} }

type ServerFilter struct {
	All            bool
	Read           gatewaytypes.Visible
	Write          gatewaytypes.Visible
	FolderID       *uuid.UUID
	Recursive      bool
	TopLevel       bool
	OrganizationID *uuid.UUID
	OnlyGlobal     bool
	Query          string
	Tags           []string
	Status         *string
	State          *string
	IDs            []uuid.UUID
	Slug           *string
	AfterTitle     *string
	AfterID        *uuid.UUID
	Limit          int
}

func queryServers(ctx context.Context, q pgtx.Querier, f ServerFilter) ([]gatewaytypes.Server, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 1000
	}
	tags := f.Tags
	if tags == nil {
		tags = []string{}
	}
	return postgres.List(ctx, q, constraints, postgres.Struct(serverRow.server), `
		WITH RECURSIVE fp AS (
			SELECT id, slug::text AS path
			FROM mcp_folders
			WHERE parent_id IS NULL
			UNION ALL
			SELECT f.id, fp.path || '/' || f.slug
			FROM mcp_folders f
			JOIN fp ON f.parent_id = fp.id
		),
		down AS (
			SELECT id
			FROM mcp_folders
			WHERE id = $5
			UNION ALL
			SELECT c.id
			FROM mcp_folders c
			JOIN down ON c.parent_id = down.id
			WHERE $6
		),
		listed AS (
			SELECT s.*, c.icon IS NOT NULL AS has_icon, c.checked_at, c.outcome, c.code, c.detail, c.duration_ms, c.truncated, c.server_info,
				COALESCE(jsonb_array_length(c.tools), 0) AS tools_count, c.tools IS NOT NULL AS snapshot,
				CASE
					WHEN s.status = 'disabled' THEN 'disabled'
					WHEN c.outcome = 'ok' THEN 'ok'
					WHEN c.outcome = 'failed' AND c.tools IS NOT NULL THEN 'failing'
					ELSE 'unknown'
				END AS state
			FROM mcp_servers s
			LEFT JOIN mcp_server_checks c ON c.server_id = s.id
		)
		SELECT k.id, k.folder_id, k.organization_id, o.slug AS org_slug, o.name AS org_name,
			COALESCE(fp.path || '/', '') || k.slug AS path, k.slug, k.title, k.description, k.tags, k.author, k.icon_url, k.icon,
			k.has_icon, k.log_payloads, k.url, k.transport, k.auth_kind, k.auth_header, k.credentials_secret_id AS secret_id, sec.name AS secret_name,
			sn.id AS secret_node_id, sn.slug AS secret_node_slug, sn.name AS secret_node_name, k.status, k.disabled_tools,
			k.everyone, k.calls, k.tools_count, k.snapshot, rfc3339(k.checked_at) AS checked_at, k.outcome, k.code, k.detail,
			k.duration_ms, k.truncated, k.server_info, rfc3339(k.created_at) AS created_at, rfc3339(k.updated_at) AS updated_at
		FROM listed k
		LEFT JOIN fp ON fp.id = k.folder_id
		LEFT JOIN nodes o ON o.id = k.organization_id
		LEFT JOIN secrets sec ON sec.id = k.credentials_secret_id
		LEFT JOIN nodes sn ON sn.id = sec.node_id
		WHERE ($1 OR (((k.organization_id IS NULL AND $3) OR k.organization_id = ANY($2))
				AND (k.status <> 'draft' OR (k.organization_id IS NULL AND $8) OR k.organization_id = ANY($7))))
			AND ($5::uuid IS NULL OR k.folder_id IN (SELECT id FROM down))
			AND (NOT $4 OR k.folder_id IS NULL)
			AND ($9::uuid IS NULL OR k.organization_id = $9)
			AND (NOT $10 OR k.organization_id IS NULL)
			AND ($11 = '' OR strpos(lower(k.slug || ' ' || k.title || ' ' || k.description || ' ' ||
				array_to_string(k.tags, ' ')), lower($11)) > 0)
			AND (cardinality($12::text[]) = 0 OR k.tags @> $12)
			AND ($13::text IS NULL OR k.status = $13)
			AND ($14::text IS NULL OR k.state = $14)
			AND ($15::uuid[] IS NULL OR k.id = ANY($15))
			AND ($16::text IS NULL OR k.slug = $16)
			AND ($17::text IS NULL OR (lower(k.title), k.id) > ($17, $18::uuid))
		ORDER BY lower(k.title), k.id
		LIMIT $19`,
		f.All, f.Read.Organizations, f.Read.Global, f.TopLevel, f.FolderID, f.Recursive, f.Write.Organizations, f.Write.Global,
		f.OrganizationID, f.OnlyGlobal, f.Query, tags, f.Status, f.State, f.IDs, f.Slug, f.AfterTitle, f.AfterID, limit)
}

func (s *ServerRepository) List(ctx context.Context, f ServerFilter) ([]gatewaytypes.Server, error) {
	return queryServers(ctx, s.db.From(ctx), f)
}

func (s *ServerRepository) Get(ctx context.Context, id uuid.UUID) (*gatewaytypes.Server, error) {
	found, err := queryServers(ctx, s.db.From(ctx), ServerFilter{All: true, IDs: []uuid.UUID{id}, Limit: 1})
	if err != nil || len(found) == 0 {
		return nil, err
	}
	return &found[0], nil
}

type NewServer struct {
	ID             uuid.UUID
	FolderID       *uuid.UUID
	OrganizationID *uuid.UUID
	CreatedBy      uuid.UUID
	Fields         gatewaytypes.ServerFields
}

func (s *ServerRepository) Insert(ctx context.Context, n NewServer) error {
	f := n.Fields
	var header *string
	if f.Auth.Kind == gatewaytypes.AuthHeader {
		header = &f.Auth.Header
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH server AS (
			INSERT INTO mcp_servers (id, folder_id, organization_id, slug, title, description, tags, author, icon_url, url,
				transport, auth_kind, auth_header, credentials_secret_id, status, created_by, icon, log_payloads)
			VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE((SELECT left(display_name, 100) FROM users WHERE id = $16), $8), $9, $10, $11, $12,
				$13, $14, $15, $16, $17, $18)
			RETURNING id
		)
		INSERT INTO mcp_server_checks (server_id)
		SELECT id
		FROM server`, n.ID, n.FolderID, n.OrganizationID, f.Slug, f.Title, f.Description, f.Tags, f.Author, f.IconURL, f.URL,
		f.Transport, f.Auth.Kind, header, f.Auth.SecretID, string(f.Status), n.CreatedBy, f.Icon, f.LogPayloads)
	return err
}

func (s *ServerRepository) Update(ctx context.Context, id uuid.UUID, f gatewaytypes.ServerFields, recheck bool) error {
	var header *string
	if f.Auth.Kind == gatewaytypes.AuthHeader {
		header = &f.Auth.Header
	}
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		WITH server AS (
			UPDATE mcp_servers
			SET title = $2, description = $3, tags = $4, author = $5, icon_url = $6, url = $7, transport = $8,
				auth_kind = $9, auth_header = $10, credentials_secret_id = $11, status = $12, icon = $14, log_payloads = $15, updated_at = now()
			WHERE id = $1
			RETURNING id
		),
		due AS (
			UPDATE mcp_server_checks
			SET next_check_at = now()
			WHERE server_id IN (SELECT id FROM server)
				AND $13
		)
		SELECT id
		FROM server`, id, f.Title, f.Description, f.Tags, f.Author, f.IconURL, f.URL, f.Transport, f.Auth.Kind, header,
		f.Auth.SecretID, string(f.Status), recheck, f.Icon, f.LogPayloads)
}

func (s *ServerRepository) Move(ctx context.Context, id uuid.UUID, folder, organization *uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		WITH server AS (
			UPDATE mcp_servers
			SET folder_id = $2, organization_id = $3, updated_at = now()
			WHERE id = $1
			RETURNING id
		),
		sets AS (
			DELETE FROM mcp_organization_servers
			WHERE server_id IN (SELECT id FROM server)
				AND ($3::uuid IS NULL OR organization_id <> $3)
		),
		due AS (
			UPDATE mcp_server_checks
			SET next_check_at = now()
			WHERE server_id IN (SELECT id FROM server)
		)
		SELECT id
		FROM server`, id, folder, organization)
}

func (s *ServerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM mcp_servers
		WHERE id = $1`, id)
}

func (s *ServerRepository) Tools(ctx context.Context, id uuid.UUID) (string, []gatewaytypes.Tool, []string, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(func(r toolsRow) toolsRow { return r }), `
		SELECT s.slug, c.tools, s.disabled_tools
		FROM mcp_servers s
		LEFT JOIN mcp_server_checks c ON c.server_id = s.id
		WHERE s.id = $1`, id)
	if err != nil || found == nil {
		return "", nil, nil, err
	}
	return found.Slug, decodeTools(found.Tools), found.Disabled, nil
}

func (s *ServerRepository) SetDisabledTools(ctx context.Context, id uuid.UUID, tools []string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE mcp_servers
		SET disabled_tools = $2, updated_at = now()
		WHERE id = $1`, id, tools)
}

func (s *ServerRepository) UsingSecrets(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return postgres.Pairs[uuid.UUID, int64](ctx, s.db.From(ctx), constraints, `
		SELECT secret_id, count(*)
		FROM (
			SELECT credentials_secret_id AS secret_id
			FROM mcp_servers
			WHERE credentials_secret_id = ANY($1)
			UNION ALL
			SELECT credentials_secret_id
			FROM mcp_proxy_settings
			WHERE credentials_secret_id = ANY($1)
		) used
		GROUP BY secret_id`, secrets)
}
