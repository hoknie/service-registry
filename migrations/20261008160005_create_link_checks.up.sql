CREATE TABLE link_checks
(
    id          uuid PRIMARY KEY,
    target_id   uuid        NOT NULL,
    status      text        NOT NULL,
    http_status integer,
    duration_ms integer     NOT NULL,
    checked_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT link_checks_target_id_fkey FOREIGN KEY (target_id) REFERENCES link_targets (id) ON DELETE CASCADE,
    CONSTRAINT link_checks_status_check CHECK (status IN (
                                                          'ok', 'redirect', 'auth_required', 'not_found',
                                                          'server_error', 'unexpected',
                                                          'timeout', 'tls_error', 'dns_error', 'unreachable', 'blocked'))
);

CREATE INDEX link_checks_target_idx ON link_checks (target_id, checked_at);
