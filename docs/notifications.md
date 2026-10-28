# Notifications

The registry can tell you when something happens: a new deployment, drift in a cluster, a broken link, a failed repository
synchronization — and when it recovers. Messages go to a **webhook** of yours or to a **Slack** channel.

## Subscribing

Subscriptions live on organizations, folders and projects and cover everything below.

1. Put the receiver's address into a [secret](secrets.md). For Slack it is an incoming webhook address
   `https://hooks.slack.com/…`.
2. Open the node, **Settings → Notifications** (admins of the node), and add a subscription:

| Field | Meaning |
|---|---|
| Key | a short name (`a-z`, `0-9`, `-`), unique on the node. A subscription with the same key lower in the tree overrides or disables this one there |
| Channel | webhook or Slack |
| Address secret | the secret with the receiver's address |
| Events | which events to send (below). A recovery can be chosen only together with its failure |
| Environments | empty means all. Link and repository sync events have no environment and always pass |
| Language | `en`, `es`, `ru` or `zh` — the language of Slack messages and of the link back to the registry |
| Disabled | sends nothing and switches off an inherited subscription with the same key |

3. Press **Test** to send a test message at once (at most once every 10 seconds).
4. The delivery log of the node shows what was sent, when, and what the receiver answered.

## Events

| Event | Sent when |
|---|---|
| `deployment.created` | a release is added: reported by CI, or seen only in a cluster. Sending the same CI event again does not notify |
| `deployment.drift` | a cluster runs another version than the current release for longer than `NOTIFY_DRIFT_AFTER_SECS` (15 minutes) |
| `deployment.drift_resolved` | the drift you were told about is gone |
| `link.check_failed` | a link that worked stops working. The first check of an address and forbidden addresses never notify |
| `link.check_recovered` | a broken link you were told about works again |
| `repo_sync.failed` | a repository sync fails after one that did not. Hitting the hosting's rate limit is not a failure |
| `repo_sync.recovered` | a synchronization succeeds after a failure you were told about |

**No repeats.** A failure opens an *episode*: the drift of one service and environment, one broken link, one repository
sync connection. While the episode is open, the same failure is not sent again. Its recovery closes it, and the next
failure is sent again.

**Recoveries.** A recovery goes only to subscriptions that received the failure and chose the recovery event.

## What a webhook receives

A `POST` with `Content-Type: application/json`:

```json
{
  "id": "0192f0c4-…",
  "type": "deployment.created",
  "occurred_at": "2026-10-23T10:00:00Z",
  "node": {"id": "…", "kind": "project", "name": "API", "path": "acme/backend/api",
           "url": "https://registry.example/en/catalog?node=…"},
  "owners": [{"subject_kind": "group", "subject_id": "…", "name": "Platform", "email": null, "status": null,
              "contacts": [], "on_call_url": "https://oncall.example/platform"}],
  "data": {"deployment_id": "…", "service": "api", "environment": "production", "version": "2.0.0",
           "commit_sha": "abcdef1", "branch": "main", "url": null, "deployed_by": null,
           "source": "event", "cluster": null, "namespace": null, "current": true}
}
```

The message is fixed when the event happens: later renames or owner changes do not change it. `node.url` is empty
without `PUBLIC_URL`. `data` by event:

| Event | `data` |
|---|---|
| `deployment.created` | the release: service, environment, version, commit, branch, source, cluster |
| `deployment.drift` | the service, environment and current version, what clusters run (`observed`) and since when |
| `deployment.drift_resolved` | the current release and when the drift started |
| `link.check_failed`, `link.check_recovered` | the link, its address, the new and the previous state |
| `repo_sync.failed`, `repo_sync.recovered` | the connection, the hosting (`hosting`), the owner, the run and its error |
| `test` | the subscription |

A recovery also carries `"recovers"` — the id of the failure message it closes.

Older versions sent `forge.sync_failed` and `forge.sync_recovered` with the field `forge` and error codes `forge.*`.
They are now `repo_sync.failed` and `repo_sync.recovered` with the field `hosting` and codes `hosting.*`
(`repo_sync.interrupted` for an interrupted run); existing subscriptions are switched over on upgrade.

## Checking the signature

Create a **signing key** for a webhook subscription; it is shown once (`whsec_…`). Requests then carry the
[Standard Webhooks](https://www.standardwebhooks.com/) headers:

- `webhook-id` — the same in every retry of one delivery: use it to drop duplicates;
- `webhook-timestamp` — Unix seconds of the attempt: refuse old ones;
- `webhook-signature` — `v1,` and a base64 HMAC-SHA256 of `<webhook-id>.<webhook-timestamp>.<body>` with the key
  (the part after `whsec_`, base64-decoded).

```python
import base64, hashlib, hmac

def valid(secret: str, headers: dict, body: bytes) -> bool:
    key = base64.b64decode(secret.removeprefix("whsec_"))
    signed = f"{headers['webhook-id']}.{headers['webhook-timestamp']}.".encode() + body
    expected = "v1," + base64.b64encode(hmac.new(key, signed, hashlib.sha256).digest()).decode()
    return any(hmac.compare_digest(expected, s) for s in headers["webhook-signature"].split(" "))
```

Standard Webhooks libraries for many languages check the same headers.

## Delivery

- **Nothing is lost.** A message is written together with the event, so a crash does not lose it and a rolled-back
  change sends nothing.
- **At least once.** Your receiver may see the same message twice, with the same `webhook-id`.
- **Success** is any `2xx` answer.
- **Retries.** A network error, a timeout, `408`, `429` or `5xx` is retried after 30 s, 2 min, 10 min, 30 min, 1 h,
  3 h, 6 h and 12 h. `Retry-After` is honoured up to an hour. Any other answer fails at once.
- **Redirects** are not followed.
- **Address checks.** Deliveries never go to loopback, link-local (cloud metadata), multicast or unspecified
  addresses, nor to hosts in `NOTIFY_DENY_HOSTS`. Private networks are allowed unless `NOTIFY_ALLOW_PRIVATE=false`
  (then only `NOTIFY_ALLOW_HOSTS`).

**Through a proxy.** By default deliveries ignore `HTTPS_PROXY`, because a proxy hides the receiver's address from
these checks. With `NOTIFY_USE_PROXY=true` they go through the proxy:
- the receiver's IP is no longer checked, so block internal networks on the proxy itself;
- `NOTIFY_DENY_HOSTS`, `localhost` and addresses written as an IP are still refused;
- hosts in `NO_PROXY` connect directly with the full check;
- the server logs a warning at start.

| Variable | Default | Meaning |
|---|---|---|
| `NOTIFY_DRIFT_AFTER_SECS` | 900 | how long drift lasts before it is reported |
| `NOTIFY_MAX_ATTEMPTS` | 8 | attempts per delivery |
| `NOTIFY_TIMEOUT_SECS` | 10 | timeout of one attempt |
| `NOTIFY_ALLOW_PRIVATE` | `true` | `false` — private-network receivers only if listed in `NOTIFY_ALLOW_HOSTS` |
| `NOTIFY_ALLOW_HOSTS`, `NOTIFY_DENY_HOSTS` | — | receivers always allowed, receivers never contacted |
| `NOTIFY_USE_PROXY` | `false` | send through `HTTPS_PROXY` |
| `NOTIFY_RETENTION_DAYS` | 30 | how long messages and the delivery log are kept |

Deliveries are sent by the server's background jobs (`BACKGROUND_JOBS_ENABLED=true`).
