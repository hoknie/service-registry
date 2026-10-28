# Repository sync

Connect an organization or folder to an owner on **GitHub, GitLab, Forgejo or Gitea** — an organization, a user or a
GitLab group — and every repository of that owner becomes a project in the catalog, kept up to date.

## Connecting

1. Create a [secret](secrets.md) with a token of the hosting. A read-only token is enough, unless the registry should
   register the webhook itself.
2. Open the organization or folder, **Settings → Repository sync**, and add a connection: the hosting, its address, the owner and
   the secret.
3. Optionally limit which branches are read (`release/*`, …) and choose whether GitLab subgroups become folders.
4. Press **Sync now**, or wait for the first scheduled run.

What the synchronization keeps current for every project:
- the description and topics;
- the README and languages;
- the default branch;
- the archived flag and visibility;
- stars and the licence;
- branches ([catalog.md](catalog.md#branches)).

Imported projects are **managed**. The synchronization creates, moves and renames them; their description and
repository fields are changed on the hosting, not in the registry. A repository that disappears marks its project
*orphaned*. Nothing is deleted.

## When it runs

- on a schedule — every `REPO_SYNC_INTERVAL_SECS` (15 minutes) by default, or the connection's own interval;
- on **Sync now**;
- on a webhook delivery from the hosting (below);
- from the command line: `svc-registry repo:sync <connection-id>`.

Several servers on one database never run the same connection twice.

## Webhooks

With a webhook, a push reaches the registry at once instead of waiting for the next scheduled run. The address is

```
<PUBLIC_URL>/api/v1/repo-sync/hooks/<connection-id>
```

Webhooks registered by older versions at `/api/v1/forge/hooks/<connection-id>` keep working; registering the
webhook again moves it to the new address.

The registry registers it on the hosting if the token may do so. Otherwise the connection shows the address and the
secret to enter on the hosting by hand. `PUBLIC_URL` must be set.

## Taking a project out of the synchronization

**Settings → General → Unlink from repository sync** turns a managed project into an ordinary one:
- the repository is added to the connection's exclusions;
- the project keeps its documentation source as a repository by address with the connection's secret.

To bring it back, remove the exclusion in the connection dialog; the next run links it again.

## Settings

| Variable | Default | Meaning |
|---|---|---|
| `REPO_SYNC_INTERVAL_SECS` | 900 | default interval of a connection |
| `BACKGROUND_JOBS_ENABLED` | `true` | `false` makes a server answer requests only, without running synchronizations |
| `HTTP_CA_FILE` | — | extra CA certificates for outgoing HTTPS, next to the system ones |
| `HTTPS_PROXY`, `NO_PROXY` | — | outgoing proxy; honoured by requests to the hosting |

More `REPO_SYNC_*` tuning: [`.env.example`](../.env.example).

The variables were called `FORGE_SYNC_*` before; a server started with one of the old names stops and names the new
one.
