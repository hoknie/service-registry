# Personal access tokens

Scripts, CI jobs and MCP clients call the API as a user with a **personal access token**
(`svcp_…`): issue one under **Account → Tokens** (shown once), pick scopes — `read` (catalog
reads), `write` (catalog changes), `admin` (superadmins only: users, groups, superadmin powers),
`mcp` — and a lifetime of 1 to `PAT_MAX_LIFETIME_DAYS` (365) days; superadmins may issue tokens
without expiry. A token never has more rights than its user, and only a signed-in session can
issue, list or revoke tokens.

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
