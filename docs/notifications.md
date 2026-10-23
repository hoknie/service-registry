# Notifications

svc-registry sends notifications about what happens in the catalog to a **webhook** or a **Slack** incoming
webhook. Subscriptions live on organizations, folders and projects and apply to everything below.

## Subscriptions

Open a node, **Settings → Notifications** (administrators of the node, `catalog.access`).

| Field | Meaning |
|---|---|
| Key | `a-z`, `0-9` and `-`, unique on the node. The same key below overrides or disables the subscription. |
| Channel | `webhook` or `slack`. |
| Address secret | A named secret holding the receiver's address. Slack accepts only `https://hooks.slack.com/…`. |
| Events | `deployment.created`, `deployment.drift`, `deployment.drift_resolved`, `link.check_failed`, `link.check_recovered`, `forge.sync_failed`, `forge.sync_recovered`. A recovery type is chosen separately from its failure type, but only together with it: `deployment.drift_resolved` needs `deployment.drift`, `link.check_recovered` needs `link.check_failed`, `forge.sync_recovered` needs `forge.sync_failed` — otherwise `400 validation.recovery_without_failure`. Subscriptions saved before this rule keep their types until they are edited. |
| Environments | Empty — all. Link and forge events have no environment and always pass. |
| Language | `en`, `es`, `ru` or `zh` — the language of Slack messages and of the node link in the payload. |
| Disabled | Receives nothing and switches off the inherited subscription with the same key. |

The same through the API: `GET/PUT/DELETE /api/v1/catalog/nodes/{id}/notification-subscriptions[/{key}]`,
`POST …/{key}/test` (one test at most every 10 seconds), `POST …/{key}/signing-secret`,
`GET /api/v1/catalog/nodes/{id}/notification-deliveries?subscription=&status=` (the delivery log).

## Events

| Type | When |
|---|---|
| `deployment.created` | A deployment is added to a project's history: an accepted `service.deployed` or a deployment seen only in a cluster (`"source": "cluster"`). A replayed event does not notify. |
| `deployment.drift` | A cluster runs a version other than the current deployment for longer than `NOTIFY_DRIFT_AFTER_SECS` (15 minutes by default). One notification per episode; a new current deployment or a new observed version starts a new one. |
| `link.check_failed` | A link address that worked (`ok`, `redirect`, `auth_required`) stops working. The first check of an address and `blocked` never notify. |
| `forge.sync_failed` | A forge sync run fails after a run that did not. `rate_limited` is not a failure. |
| `deployment.drift_resolved` | No cluster runs another version than the current deployment of a pair whose drift was notified. |
| `link.check_recovered` | A notified broken link answers with a working status again. |
| `forge.sync_recovered` | A run succeeds after a notified `forge.sync_failed`. `rate_limited` does not close it. |

### Episodes and recoveries

A failure notification opens an **episode** of its subject: the service and environment of a project (drift), the
address and link of a project (link) or the forge connection. While the episode is open the same failure does
not notify again; its recovery closes it, and the next failure notifies again even when it looks the same as
last time. A recovery is sent only by subscriptions that received the last failure of the episode **and** list the
recovery type — a recovery without a notified failure is never sent. An episode older than
`NOTIFY_RETENTION_DAYS` (its failure event is deleted) closes silently.

## Payload

Webhooks receive `POST` with `Content-Type: application/json`:

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

`node.url` is `null` without `PUBLIC_URL`. The payload is fixed when the event happens: later renames and owner
changes do not change it. `data` by type:

- `deployment.drift` — `service`, `environment`, `version`, `deployment_id`, `observed` (`[{cluster, namespace, version}]`), `since`;
- `link.check_failed` — `link_key`, `kind_key`, `title`, `url`, `status`, `http_status`, `previous_status`, `checked_at`;
- `forge.sync_failed` — `connection_id`, `forge`, `owner_path`, `run_id`, `trigger`, `error_code`, `error_message`;
- `deployment.drift_resolved` — `service`, `environment`, `version`, `deployment_id` of the current deployment, `since` of the drift;
- `link.check_recovered` — as `link.check_failed`, with the working `status` and the broken `previous_status`;
- `forge.sync_recovered` — as `forge.sync_failed` of the successful run, `error_code` and `error_message` are `null`;
- `test` — `subscription`.

A recovery payload also carries `"recovers": "<id of the failure event>"`.

## Checking the signature

Create a signing key for a webhook subscription (it is shown once, `whsec_…`). Requests then carry the
[Standard Webhooks](https://www.standardwebhooks.com/) headers:

- `webhook-id` — the delivery id, the same in every attempt: use it to drop duplicates;
- `webhook-timestamp` — Unix seconds of the attempt: refuse old ones;
- `webhook-signature` — `v1,` + base64 of HMAC-SHA256 of `<webhook-id>.<webhook-timestamp>.<body>` with the key
  (the base64 part after `whsec_`, decoded).

```python
import base64, hashlib, hmac

def valid(secret: str, headers: dict, body: bytes) -> bool:
    key = base64.b64decode(secret.removeprefix("whsec_"))
    signed = f"{headers['webhook-id']}.{headers['webhook-timestamp']}.".encode() + body
    expected = "v1," + base64.b64encode(hmac.new(key, signed, hashlib.sha256).digest()).decode()
    return any(hmac.compare_digest(expected, s) for s in headers["webhook-signature"].split(" "))
```

Libraries of Standard Webhooks for many languages check the same headers.

## Delivery

- Deliveries are written together with the fact, so a notification is never lost on a crash and never appears
  for a rolled-back fact. They are sent by a background job of `serve` (`BACKGROUND_JOBS_ENABLED=true`).
- The guarantee is **at least once**: a process may stop after the receiver answered `2xx` and before the result is
  written, then the delivery is sent again with the same `webhook-id`.
- `2xx` — delivered. A network error, a timeout, `408`, `429` or `5xx` is retried after 30 s, 2 min, 10 min, 30 min,
  1 h, 3 h, 6 h and 12 h (`NOTIFY_MAX_ATTEMPTS`); `Retry-After` of `429`/`503` extends a pause up to 1 h. Any other
  answer fails at once. Redirects are not followed (`notify.redirect`).
- The receiver's address is resolved before each connection and refused when it is loopback, link-local (cloud
  metadata), multicast or unspecified, a private network with `NOTIFY_ALLOW_PRIVATE=false` (unless listed in
  `NOTIFY_ALLOW_HOSTS`), or listed in `NOTIFY_DENY_HOSTS` — `notify.blocked`. By default deliveries do not use
  `HTTPS_PROXY`/`HTTP_PROXY`: with a proxy the policy could not see the receiver's address.
- **Through a proxy** (`NOTIFY_USE_PROXY=true`, for networks that reach the outside only through one): deliveries
  and test sends go through `HTTPS_PROXY`/`HTTP_PROXY`. The proxy resolves the receiver's name, so **the
  receiver's IP is not checked**: a name that resolves to an internal address is not caught — block internal
  networks on the proxy itself. What stays: `NOTIFY_DENY_HOSTS`, `localhost`/`*.localhost`, numeric hosts and
  addresses written as an IP in the URL are refused before the request (`notify.blocked`); hosts in `NO_PROXY`
  connect directly with the full check; redirects are not followed. `serve` logs a warning at start.
- Events and deliveries older than `NOTIFY_RETENTION_DAYS` (30 by default) are deleted hourly.
