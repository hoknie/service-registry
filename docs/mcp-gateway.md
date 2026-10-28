# MCP gateway

The registry can be the one MCP server your agents connect to: it keeps a catalog of external MCP servers (trackers,
wikis, monitoring and others) and offers their tools through its own endpoint `/api/mcp` ([mcp.md](mcp.md)). The agent
never sees the external servers' addresses or tokens — the registry calls them with credentials it keeps.

The gateway is on by default. `MCP_GATEWAY_ENABLED=false` turns it off: the **MCP** page, its API and the gateway tools
disappear, the stored servers and settings stay and come back when it is turned on again.

## The catalog

**MCP** in the sidebar opens the catalog: spaces (global and one per organization), folders and servers, shown like
the service catalog — breadcrumbs with the neighbours and contents of a level, **Contents** as cards, a table or a tree.
A server card has the title, description, tags, state, number of tools and calls. A folder has **Settings** with
**General** (name, slug, description, deleting) and **Proxy**; an organization's space has the same MCP settings as the
organization. Like skills ([skills.md](skills.md)), every top-level
folder and server has a **scope**:

| Scope | Who reads | Who changes | Who decides who may use it |
|---|---|---|---|
| Global | everyone who is signed in | superadmins and members of groups with the **Global MCP servers** right ([access.md](access.md#administration-rights-for-groups)) | the same |
| An organization | everyone who sees the organization | editors and admins of the organization, its owners, superadmins | admins of the organization, superadmins |

A server is a **draft** (only those who can change it see it and its tools), **published** or **disabled** (kept, not
checked, no tools).

## The registry's own server

The first card of the **Global** space is **Service Registry**, marked **Built-in**: the registry's own MCP tools on
`/api/mcp` — `list_projects`, `get_doc`, `get_skill`, `get_guide` and the others. It cannot be moved, deleted or checked,
and its tools have no prefix. Its page has **Connect**, **Overview** and **Tools**.

On **Tools**, a superadmin or a holder of the `mcp` right can switch any built-in tool off. A switched-off tool is gone
from `tools/list` for every user, token and app, and calling it fails as an unknown tool. The list applies while the
gateway is on; with `MCP_GATEWAY_ENABLED=false` every built-in tool is available again.

## Adding a server

**Add → Server** asks for:

| Field | Meaning |
|---|---|
| Slug | `a-z`, `0-9` and `-`, up to 32, unique in the whole catalog; the server's tools are named `<slug>__<tool>`, so it cannot change later |
| Address and transport | the server's URL: Streamable HTTP (most servers) or SSE |
| Authentication | none, a bearer token (`Authorization: Bearer …`) or a header of your choice; the value comes from a named secret ([secrets.md](secrets.md)) — of the organization or a global one, global servers use global secrets only |
| Icon | **Automatically** — the icon the server declares in MCP or its `favicon.ico`, fetched at every check and kept by the registry; **From the set** — an interface icon; **By address** — an `https://` image |

The author of a server is the person who added it; it is set by the registry and cannot be changed.

**Check connection** saves the server as a draft and connects to it at once. The registry checks every server every
`MCP_GATEWAY_CHECK_INTERVAL_SECS` (5 minutes) and right after its address, authentication or proxy change: it keeps
the server's description and the list of its tools (at most `MCP_GATEWAY_MAX_TOOLS`). **Check now** on the server page
does it again.

| State | Meaning |
|---|---|
| Working | the last check succeeded |
| Error | the last check failed; the tools of the last successful check are still offered |
| Not checked | no check has succeeded yet; no tools are offered |
| Disabled | the server is disabled |

A failed check shows its code: `gateway.unreachable`, `gateway.timeout`, `gateway.unauthorized` (the server refused
the token), `gateway.blocked` (the address is not allowed, see below), `gateway.proxy`, `gateway.protocol` (the answer
is not MCP) or `gateway.secret_unavailable`.

On the server page **Settings** has sections: **General** (with
deleting at the bottom), **Connection** (address, transport, authentication), **Proxy** and **Access**.

**Tools** on the server page lists them with their schemas. Turn off the ones agents must not call, for example the
ones that delete data.

## Proxies and addresses

How the registry reaches a server is set on the organization (**Settings → MCP** of its space in the MCP section), on a folder and on a server; the
nearest setting wins, from the server up through its folders to its organization:

- **Direct** — the registry connects itself.
- **Environment** — through `HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY` of the registry process.
- **Proxy** — through the given `http://` or `https://` proxy, with hosts that bypass it and, if needed, a secret with
  `user:password` for it.

Without a setting servers connect directly, or through the environment's proxy with `MCP_GATEWAY_USE_PROXY=true`.

Connecting directly, the registry refuses private, loopback and link-local addresses unless the host or network is
listed in `MCP_GATEWAY_ALLOW_HOSTS`, e.g. `10.0.0.0/8,mcp.internal,*.corp.example`. Only the administrator of the
registry can open internal networks this way. Redirects are never followed.

## Who gets which servers

Three settings decide which servers — and so which tools — an agent gets; a server must pass all of them:

1. **Access list of the server** (**Settings → Access** on the server page): everyone in its scope, or chosen groups and users.
   Organization servers accept global groups and groups of their organization. **Move** in the danger zone of a folder's or server's **General** settings moves it to another folder,
organization or the global space; the global space needs the global **mcp** right.

**Settings → Access** of a space or folder says who sees it: everyone with a role in the organization (and its owners)
sees the space and everything in it, editors and above change it; the roles are edited right there and are the same as
the organization's access in the catalog. The global space is seen by every signed-in user and changed by superadmins and
groups with the global **mcp** right.
2. **Servers of the organization** (**Settings → MCP** of the organization's space in the MCP section): all available servers or only chosen ones.
   It applies to tokens and apps limited to nodes of that organization ([access-tokens.md](access-tokens.md#tokens-for-selected-nodes)).
3. **Servers of the token**: all available, or chosen ones, set when the token is issued or later.

**MCP → Visibility** shows all of it in one table — servers against organizations — for the organizations you
administer, and changes access lists and organization sets in place.

### Pinning servers to a token

When issuing a token with the `mcp` scope, choose **MCP servers → Selected** and pick servers. Later, **MCP servers**
in the token's menu on the account page changes the list — the token keeps working, nothing has to be copied again,
and the new list applies from the agent's next request. The same is available for apps connected over OAuth (the
**Apps** tab) and on the consent page. A deleted server disappears from all lists.

Over the API: `PUT /api/v1/account/tokens/<id>/mcp-servers` with `{"servers": ["<server id>", ...]}` or
`{"servers": null}` for all available servers; `PUT /api/v1/account/apps/<id>/mcp-servers` for apps.

## What agents see

In `tools/list` an agent gets the registry's own tools, `list_mcp_servers` and the tools of its servers as
`<slug>__<tool>`, e.g. `jira__search`. A call is passed to the server unchanged, with the registry's credentials; the
caller's token and headers are not passed on. A failed call returns an error result starting with its code (see
above; also `gateway.result_too_large` above `MCP_GATEWAY_MAX_RESULT_BYTES`). A call is cut after
`MCP_GATEWAY_CALL_TIMEOUT_SECS` (60 s). Only tools are passed through — resources and prompts of external servers are
not.

**Connect** on the server page shows the registry's MCP address and example settings for agents.

## Call log

**Log** on the server page lists the calls: when, which tool, who and through which token or app, how long and the
outcome; the tool name opens it on **Tools**. Open a row to see the request (the arguments) and the response of the
external server. They are kept while **Record arguments and responses in the call log** is on in the server's **Settings →
General** (on by default): each up to 16 KiB, with the values of the server's and proxy's secrets cut out. Turn it off for
servers whose calls carry personal or secret data; records written before stay until they expire. Records older than `MCP_GATEWAY_CALL_LOG_DAYS` (30) are deleted.
Metrics of the gateway are described in [metrics.md](metrics.md).
