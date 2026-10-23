# Personal access tokens

Scripts, CI jobs and MCP clients call the API as a user with a **personal access token**
(`svcp_…`): issue one under **Account → Tokens** (shown once), pick scopes — `read` (catalog
reads), `write` (catalog changes), `admin` (superadmins only: users, groups, superadmin powers),
`mcp` — and a lifetime of 1 to `PAT_MAX_LIFETIME_DAYS` (365) days; superadmins may issue tokens
without expiry. A token never has more rights than its user, and only a signed-in session can
issue, list or revoke tokens.

**Superadmin tokens without `admin`.** A token a superadmin issues without `admin` acts as a catalog
administrator — on the whole catalog, or in the chosen nodes (below) — while its owner stays a
superadmin; creating top-level organizations and global administration (users, groups, clusters,
environments, link kinds, global secrets) still need `admin`. The token list marks such tokens
**catalog admin** (`"catalog_admin": true`). Tokens issued before this rule (or while the owner was not a
superadmin) keep only the owner's role bindings; issue a new token to get the catalog role.

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
read each node when issuing. A token for nodes has only `read`, `write` and `mcp`: the dialog disables
`admin` while **Selected nodes only** is chosen, and the API refuses the combination
(`400 validation.invalid_token_nodes`). A superadmin's token for nodes acts as a catalog administrator
in the chosen subtrees (bindings, project keys, node secrets, notifications, owners) without any
global superadmin power; it loses that as soon as the superadmin flag is removed. Your own groups
(`GET /api/v1/account/groups`) need `read`; editing a group's contacts as its contact editor needs
`write` and a token without nodes, since a group is not part of the catalog. A moved node keeps limiting its subtree; a deleted one opens
nothing, and the token never becomes unrestricted. To change the nodes, issue a new token. Spec:
`openspec/specs/access/authorization/` («Узлы токена»).
