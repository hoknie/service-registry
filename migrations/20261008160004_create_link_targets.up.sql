CREATE TABLE link_targets
(
    id               uuid PRIMARY KEY,
    url              text        NOT NULL,
    next_run_at      timestamptz NOT NULL DEFAULT now(),
    lease_until      timestamptz,
    last_status      text,
    last_http_status integer,
    last_duration_ms integer,
    last_checked_at  timestamptz,
    last_seen_at     timestamptz NOT NULL DEFAULT now(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT link_targets_url_key UNIQUE (url),
    CONSTRAINT link_targets_last_status_check CHECK (last_status IN (
                                                                     'ok', 'redirect', 'auth_required', 'not_found',
                                                                     'server_error', 'unexpected',
                                                                     'timeout', 'tls_error', 'dns_error', 'unreachable',
                                                                     'blocked'))
);

CREATE INDEX link_targets_next_run_idx ON link_targets (next_run_at);
CREATE INDEX link_targets_last_seen_idx ON link_targets (last_seen_at);
