# Personal access tokens

Scripts, CI jobs and MCP clients call the API as a user with a **personal access token**
(`svcp_…`): issue one under **Account → Tokens** (shown once), pick scopes — `read` (catalog
reads), `write` (catalog changes), `mcp` — and a lifetime of 1 to `PAT_MAX_LIFETIME_DAYS` (365)
days; superadmins may issue tokens without expiry. A token never has more rights than its user, and only a signed-in session can
issue, list or revoke tokens.

**Global rights.** Superadmins may add global rights, each opening one part of the administration and
nothing else. Every part has a read right (`GET`/`HEAD`) and a change right (other methods); the change
right does not include reading, and the dialog ticks both when you pick a change:

| Rights | Opens |
|---|---|
| `users:read`, `users:write` | `/api/v1/users…`: users, their identities and groups (their tokens and apps stay session-only) |
| `groups:read`, `groups:write` | `/api/v1/groups…`: groups, members, contacts, contact editors |
| `clusters:read`, `clusters:write` | `/api/v1/clusters…`: clusters and agent tokens |
| `settings:read`, `settings:write` | `/api/v1/secrets…`, `/api/v1/knowledge/scans`; changes of `/api/v1/link-kinds…` and `/api/v1/environments…` (`settings:write`; reading them needs `read`) |
| `catalog.root` | creating, moving and deleting top-level organizations (with `write`) |

A global right works only while the owner is a superadmin; only `catalog.root` may be combined with nodes
(below). The former `admin` scope and the undivided `users`, `groups`, `clusters`, `settings` are gone:
existing tokens got both halves of each when the database was migrated, and a request with one of the
old names in `scopes` is `400 validation.invalid_scopes`.

**Superadmin tokens and the catalog.** A token a superadmin issues acts as a catalog administrator — on
the whole catalog, or in the chosen nodes (below) — while its owner stays a superadmin; the top level of
the catalog needs `catalog.root`. The token list marks such tokens **catalog admin** (`"catalog_admin":
true`, computed from the owner's current flag). Tokens issued before this rule (or while the owner was not
a superadmin) keep only the owner's role bindings; issue a new token to get the catalog role.

```sh
curl -H "Authorization: Bearer $SVCR_TOKEN" "$SVCR_URL/api/v1/catalog/nodes"
```

Any bad token — malformed, revoked, expired, a disabled user's, a project key — is
`401 auth.invalid_token`; a missing scope is `403 auth.insufficient_scope`. Superadmins find
leaked tokens by prefix under **Administration → Tokens**. Spec:
`openspec/specs/access/personal-access-tokens/`.

**Service accounts.** For integrations, create a user with "Service account" (password optional)
under **Administration → Users**, or turn it on for an existing user. A service account cannot sign
in with a password or through a login provider (turning it on closes its sessions); it works only
with tokens, which a superadmin issues on the user's page (**Issue token**, `POST
/api/v1/users/{id}/tokens`, shown once). Its rights are its roles and groups, like any user. Spec:
`openspec/specs/access/service-users/`.

**Tokens for selected nodes.** Under **Catalog access** pick **Selected nodes only** and find up to
20 organizations, folders or projects (`"nodes": ["<id>", …]` in the request body). Such a token works
only inside those subtrees: a role you hold on an ancestor applies to the chosen node and below,
ancestors are visible only as navigation stubs, everything else answers `404`. You must be able to
read each node when issuing. A token for nodes has only `read`, `write`, `mcp` and `catalog.root`: the dialog
disables the other global rights while **Selected nodes only** is chosen, and the API refuses them
(`400 validation.invalid_token_nodes`). With `catalog.root`, a superadmin's token for nodes creates
top-level organizations, and each one it creates joins its nodes in the same transaction (up to 20; then
`409 conflict.token_nodes_full` and nothing is created); it moves to or from the top level and deletes
only top-level organizations among its nodes. A superadmin's token for nodes acts as a catalog administrator
in the chosen subtrees (bindings, project keys, node secrets, notifications, owners) without any
global superadmin power; it loses that as soon as the superadmin flag is removed. Your own groups
(`GET /api/v1/account/groups`) need `read`; editing a group's contacts as its contact editor needs
`write` and a token without nodes, since a group is not part of the catalog. A moved node keeps limiting its subtree; a deleted one leaves the
token's nodes (and the nodes of MCP app grants), so it no longer takes one of the 20 places. When every node of a token
is deleted, the first one stays listed as a node that no longer exists: it opens nothing, and the token never becomes
unrestricted. To change the nodes, issue a new token. Spec:
`openspec/specs/access/authorization/` («Узлы токена»).
