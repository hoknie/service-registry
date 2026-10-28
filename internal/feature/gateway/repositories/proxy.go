package repositories

import (
	"context"

	"github.com/google/uuid"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ProxyRepository struct{ db *pgtx.DB }

func NewProxyRepository(db *pgtx.DB) *ProxyRepository { return &ProxyRepository{db: db} }

type ProxyStart struct {
	Server       *uuid.UUID
	Folder       *uuid.UUID
	Organization *uuid.UUID
}

func (s *ProxyRepository) Chain(ctx context.Context, from ProxyStart) ([]gatewaytypes.ProxyLink, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(proxyRow.link), `
		WITH RECURSIVE folders AS (
			SELECT f.id, f.parent_id, f.name, 1 AS lvl
			FROM mcp_folders f
			WHERE f.id = COALESCE($2, (SELECT folder_id FROM mcp_servers WHERE id = $1))
			UNION ALL
			SELECT p.id, p.parent_id, p.name, folders.lvl + 1
			FROM mcp_folders p
			JOIN folders ON p.id = folders.parent_id
		),
		owners AS (
			SELECT 'server' AS owner_kind, s.id AS owner_id, s.title AS owner_name, 0 AS lvl
			FROM mcp_servers s
			WHERE s.id = $1
			UNION ALL
			SELECT 'folder', id, name, lvl
			FROM folders
			UNION ALL
			SELECT 'organization', o.id, o.name, 1000
			FROM nodes o
			WHERE o.id = COALESCE($3, (SELECT organization_id FROM mcp_servers WHERE id = $1),
				(SELECT organization_id FROM mcp_folders WHERE id = $2))
		)
		SELECT ow.owner_kind, ow.owner_id, ow.owner_name, p.mode, p.url, p.no_proxy, p.credentials_secret_id AS secret_id,
			sec.name AS secret_name, sn.id AS secret_node_id, sn.slug AS secret_node_slug, sn.name AS secret_node_name
		FROM owners ow
		LEFT JOIN mcp_proxy_settings p ON (ow.owner_kind = 'server' AND p.server_id = ow.owner_id)
			OR (ow.owner_kind = 'folder' AND p.folder_id = ow.owner_id)
			OR (ow.owner_kind = 'organization' AND p.organization_id = ow.owner_id)
		LEFT JOIN secrets sec ON sec.id = p.credentials_secret_id
		LEFT JOIN nodes sn ON sn.id = sec.node_id
		ORDER BY ow.lvl`, from.Server, from.Folder, from.Organization)
}

func (s *ProxyRepository) Delete(ctx context.Context, from ProxyStart) (bool, error) {
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM mcp_proxy_settings
		WHERE server_id = $1
			OR folder_id = $2
			OR organization_id = $3`, from.Server, from.Folder, from.Organization)
	return n > 0, err
}

func (s *ProxyRepository) Insert(ctx context.Context, id uuid.UUID, from ProxyStart, p gatewaytypes.Proxy) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO mcp_proxy_settings (id, server_id, folder_id, organization_id, mode, url, no_proxy, credentials_secret_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, id, from.Server, from.Folder, from.Organization, p.Mode, p.URL, p.NoProxy, p.SecretID)
	return err
}

func (s *ProxyRepository) Requeue(ctx context.Context, from ProxyStart) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH RECURSIVE down AS (
			SELECT id
			FROM mcp_folders
			WHERE id = $2
			UNION ALL
			SELECT c.id
			FROM mcp_folders c
			JOIN down ON c.parent_id = down.id
		)
		UPDATE mcp_server_checks c
		SET next_check_at = now()
		FROM mcp_servers s
		WHERE s.id = c.server_id
			AND (s.id = $1 OR s.folder_id IN (SELECT id FROM down) OR s.organization_id = $3)`, from.Server, from.Folder, from.Organization)
	return err
}
