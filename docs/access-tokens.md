# Personal access tokens

Scripts, CI jobs and MCP clients act as a user with a **personal access token** (`svcp_…`). A token never has more
rights than its user.

## Issuing a token

1. Open **Account → Tokens** and press the issue button. Only a signed-in browser session can issue, list or revoke
   tokens.
2. Pick the scopes (below), the lifetime — 1 to `PAT_MAX_LIFETIME_DAYS` (365) days — and, if you like, the catalog
   nodes the token is limited to.
3. Copy the token: it is shown once.

Superadmins may issue tokens without expiry. Service accounts get their tokens from a superadmin
([access.md](access.md#users)).

Use it in the `Authorization` header:

```sh
curl -H "Authorization: Bearer $SVCR_TOKEN" "$SVCR_URL/…"
```

## Scopes

| Scope | Allows |
|---|---|
| `read` | reading the catalog: nodes, deployments, links, documentation, your own groups |
| `write` | changing the catalog within your roles |
| `mcp` | the MCP server ([mcp.md](mcp.md)) |

**Global rights** open one part of the administration each. Every part has a read right and a change right. The
change right does not include reading, so the dialog ticks both when you pick a change.

| Rights | Opens |
|---|---|
| `users:read`, `users:write` | users, their sign-in methods and groups |
| `groups:read`, `groups:write` | groups, members, contacts, contact editors |
| `clusters:read`, `clusters:write` | clusters and agent tokens |
| `settings:read`, `settings:write` | global secrets and scans; changes of link kinds and environments |
| `catalog.root` | creating, moving and deleting organizations at the top of the catalog (with `write`) |
| `publication:write` | publication settings of documentation (with `read`) |
| `skills:write` | global AI skills, their folders and git sources (with `write`) |
| `mcp_servers:write` | global MCP servers, their folders, proxies and access lists (with `write`) |

Who may get them:
- Superadmins may add any of them.
- Other users only for parts their groups have rights in ([access.md](access.md#administration-rights-for-groups)).
  `:read` needs read in one of the part's sections; `:write` needs create, update or delete.
- `catalog.root` is for superadmins and for people whose groups may create, move or delete on **Catalog top level**;
  reading the top level alone is not enough. That reading works through the `read` scope, and not in a token limited to
  selected nodes.
- A global right works only while the owner still is a superadmin or still holds the group right, and only for the
  exact actions the owner holds: with `clusters:write` and the right to update clusters only, the token updates clusters
  but cannot create or delete them. `:read` never changes anything. Managing all users' tokens works only in a browser
  session.

Tokens of older versions with the scope `admin` were converted to all global rights when the database was migrated.

**Superadmin tokens in the catalog.** A superadmin's token acts as a catalog administrator — everywhere, or in the
chosen nodes — while its owner stays a superadmin. The token list marks such tokens **catalog admin**. Tokens issued
before this rule only have the owner's own role bindings; issue a new one to get the catalog role.

## Tokens for selected nodes

Under **Catalog access** pick **Selected nodes only** and choose up to 20 organizations, folders or projects.

- The token works only inside those subtrees. A role you hold higher up applies to the chosen node and below.
- You must be able to read each node when you issue the token.
- Such a token has only `read`, `write`, `mcp` and `catalog.root`. The dialog disables the other global rights.
- With `catalog.root`, a token of a superadmin or of someone with rights on **Catalog top level** creates top-level
  organizations, and each one joins the token's nodes (up to 20). It moves and deletes only top-level organizations
  among its nodes, and only with the matching right.
- Editing a group's contacts as its contact editor needs a token without nodes, since a group is not part of the
  catalog.
- A moved node keeps limiting its subtree. A deleted node leaves the token and frees its place.
- If every node of a token is deleted, the token opens nothing; it never becomes unrestricted.
- To change the nodes, issue a new token.

## MCP servers of a token

A token with the `mcp` scope reaches external MCP servers through the registry ([mcp-gateway.md](mcp-gateway.md)). By
default it gets all servers available to its owner; **MCP servers → Selected** in the dialog limits it to chosen ones.
**MCP servers** in the token's menu changes the list at any time without issuing a new token.

## When a token stops working

A malformed, revoked, expired or disabled user's token is refused, and a missing scope too. Superadmins find leaked
tokens by their prefix under **Administration → Access → Tokens** and revoke them there.
