# Secrets

Tokens and passwords that the registry needs to reach other systems are kept as **named secrets**. A record that
needs one — a repository sync connection, a documentation source, a cluster, a notification address — picks a secret by name
instead of holding the value itself. Change the value once, and every record using it follows.

## Where secrets live

| Place | Who manages it | Who may use it |
|---|---|---|
| **Settings → Secrets** of an organization or folder | admins of the node | records of that node and everything below it |
| **Administration → Secrets → Global secrets** | superadmins and groups with rights on **Global secrets** | any record; clusters may use global secrets only |

The list of a node also shows the secrets of its ancestors, each marked with where it comes from. Global secrets appear
in a node's list only to those who can configure the node — an editor of the node and above, or a holder of **Global
secrets: read** — so a plain reader of a node (including any signed-in reader of an open organization) never sees the
names of the registry's global infrastructure secrets. The value is never shown to anyone.

## Creating a secret

1. Open the place from the table above and add a secret.
2. Give it a name and a description. Every reader of the node sees the description, never the value.
3. Choose how the value is kept:
   - **stored** — encrypted with AES-256-GCM in the database (needs `SECRETS_KEYS`);
   - **`env:NAME`** — read from the server's environment variable `NAME`;
   - **`file:/path`** — read from a file on the server, so no secret lives in the database.

The registry never shows a stored value again: only how it is kept and a fingerprint, so you can tell two values
apart.

A secret that is in use cannot be deleted. Its page shows how many records use it.

## Encryption keys

| Variable | Default | Meaning |
|---|---|---|
| `SECRETS_KEYS` | — | comma-separated `id:base64-key` pairs, e.g. `k1:$(openssl rand -base64 32)`; the first one encrypts, all of them decrypt |

To rotate: put a new key first, keep the old ones, restart, then run `svc-registry secrets:rotate`. It re-encrypts
everything with the first key; after that the old keys can go.

## Upgrading from inline tokens

Old versions kept tokens inside repository sync connections, documentation sources and clusters. They are moved into named
secrets during the upgrade; see [install.md](install.md#upgrading-from-a-version-with-inline-tokens).
