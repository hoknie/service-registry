package tests

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDeletedMCPServersLeaveTokenLists(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	user, client := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO users (id, email, display_name, password_hash) VALUES ($1, 'a@example.com', 'A', 'h')", user)
	jira, wiki := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	for slug, id := range map[string]uuid.UUID{"jira": jira, "wiki": wiki} {
		execSQL(t, tdb, "INSERT INTO mcp_servers (id, slug, title, url) VALUES ($1, $2, $2, 'https://mcp.example/'||$2)", id, slug)
	}
	first, second := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, insertToken, first, user, "{mcp}")
	execSQL(t, tdb, insertToken, second, user, "{mcp,mcp_servers:write}")
	execSQL(t, tdb, "UPDATE personal_access_tokens SET mcp_server_ids = $2 WHERE id = $1", first, []uuid.UUID{wiki, jira})
	execSQL(t, tdb, "UPDATE personal_access_tokens SET mcp_server_ids = $2 WHERE id = $1", second, []uuid.UUID{jira})
	execSQL(t, tdb, "INSERT INTO oauth_clients (id, redirect_uris) VALUES ($1, '{http://localhost/cb}')", client)
	grant := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO oauth_grants (id, user_id, client_id, scopes, refresh_hash, refresh_expires_at, mcp_server_ids) "+
		"VALUES ($1, $2, $3, '{mcp}', $4, now(), $5)", grant, user, client, []byte(strings.Repeat("h", 32)), []uuid.UUID{jira, wiki})
	execSQL(t, tdb, "DELETE FROM mcp_servers WHERE id = $1", jira)
	eq(t, scalar[string](t, tdb, "SELECT mcp_server_ids::text FROM personal_access_tokens WHERE id = $1", first), "{"+wiki.String()+"}")
	eq(t, scalar[string](t, tdb, "SELECT mcp_server_ids::text FROM personal_access_tokens WHERE id = $1", second), "{}")
	eq(t, scalar[string](t, tdb, "SELECT mcp_server_ids::text FROM oauth_grants WHERE id = $1", grant), "{"+wiki.String()+"}")
}

func TestMCPServerConstraintsHoldWithoutTheService(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	insert := "INSERT INTO mcp_servers (id, slug, title, url, auth_kind) VALUES ($1, $2, 'T', $3, $4)"
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), "Jira", "https://x", "none"), "mcp_servers_slug_check")
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), "jira", "ftp://x", "none"), "mcp_servers_url_check")
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), "jira", "https://x", "bearer"), "mcp_servers_auth_check")
	execSQL(t, tdb, insert, uuid.Must(uuid.NewV7()), "jira", "https://x", "none")
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), "jira", "https://y", "none"), "mcp_servers_slug_key")
	contains(t, execErr(t, tdb, "INSERT INTO mcp_proxy_settings (id, mode) VALUES ($1, 'direct')", uuid.Must(uuid.NewV7())),
		"mcp_proxy_settings_owner_check")
}
