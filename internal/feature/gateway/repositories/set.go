package repositories

import (
	"context"

	"github.com/google/uuid"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SetRepository struct{ db *pgtx.DB }

func NewSetRepository(db *pgtx.DB) *SetRepository { return &SetRepository{db: db} }

type availableRow struct {
	serverRow
	Tools []byte `db:"tools"`
}

func (r availableRow) available() gatewaytypes.Available {
	return gatewaytypes.Available{Server: r.server(), Tools: decodeTools(r.Tools)}
}

func (s *SetRepository) Available(ctx context.Context, v gatewaytypes.Viewer, slug *string) ([]gatewaytypes.Available, error) {
	var orgs, servers []uuid.UUID
	if v.Organizations != nil {
		orgs = v.Organizations
	}
	if v.Limited {
		servers = v.Servers
		if servers == nil {
			servers = []uuid.UUID{}
		}
	}
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(availableRow.available), `
		SELECT k.id, k.folder_id, k.organization_id, NULL::text AS org_slug, NULL::text AS org_name, k.slug AS path, k.slug,
			k.title, k.description, k.tags, k.author, k.icon_url, k.icon, c.icon IS NOT NULL AS has_icon, k.log_payloads, k.url, k.transport, k.auth_kind, k.auth_header,
			k.credentials_secret_id AS secret_id, NULL::text AS secret_name, NULL::uuid AS secret_node_id,
			NULL::text AS secret_node_slug, NULL::text AS secret_node_name, k.status, k.disabled_tools, k.everyone, k.calls,
			COALESCE(jsonb_array_length(c.tools), 0) AS tools_count, c.tools IS NOT NULL AS snapshot,
			rfc3339(c.checked_at) AS checked_at, c.outcome, c.code, c.detail, c.duration_ms, c.truncated, c.server_info,
			rfc3339(k.created_at) AS created_at, rfc3339(k.updated_at) AS updated_at, c.tools
		FROM mcp_servers k
		JOIN mcp_server_checks c ON c.server_id = k.id
		WHERE k.status <> 'disabled'
			AND c.tools IS NOT NULL
			AND (k.status = 'published' OR (k.organization_id IS NULL AND $4) OR k.organization_id = ANY($3))
			AND (k.organization_id IS NULL OR k.organization_id = ANY($2))
			AND (k.everyone OR EXISTS (
				SELECT 1
				FROM mcp_server_access a
				WHERE a.server_id = k.id
					AND (a.user_id = $1 OR a.group_id IN (SELECT gm.group_id FROM group_members gm WHERE gm.user_id = $1))
			))
			AND ($5::uuid[] IS NULL OR EXISTS (
				SELECT 1
				FROM unnest($5::uuid[]) AS org(id)
				LEFT JOIN mcp_organizations mo ON mo.organization_id = org.id
				WHERE (k.organization_id IS NULL OR k.organization_id = org.id)
					AND (COALESCE(mo.set_mode, 'all') = 'all' OR EXISTS (
						SELECT 1
						FROM mcp_organization_servers os
						WHERE os.organization_id = org.id
							AND os.server_id = k.id
					))
			))
			AND ($6::uuid[] IS NULL OR k.id = ANY($6))
			AND ($7::text IS NULL OR k.slug = $7)
		ORDER BY k.slug`, v.UserID, v.Read, v.Write.Organizations, v.Write.Global, orgs, servers, slug)
}
