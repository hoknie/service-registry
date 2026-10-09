CREATE TABLE forge_connections
(
    id                      uuid PRIMARY KEY,
    node_id                 uuid        NOT NULL,
    kind                    text        NOT NULL,
    api_url                 text        NOT NULL,
    owner_path              text        NOT NULL,
    mirror_subgroups        boolean     NOT NULL DEFAULT false,
    include_archived        boolean     NOT NULL DEFAULT false,
    include_forks           boolean     NOT NULL DEFAULT false,
    name_include            text[]      NOT NULL DEFAULT '{}',
    name_exclude            text[]      NOT NULL DEFAULT '{}',
    interval_secs           integer     NOT NULL,
    credentials_enc         text,
    credentials_ref         text,
    credentials_fingerprint text,
    webhook_mode            text,
    webhook_secret_enc      text,
    webhook_id              text,
    next_run_at             timestamptz NOT NULL DEFAULT now(),
    next_trigger            text        NOT NULL DEFAULT 'schedule',
    lease_until             timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT forge_connections_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT forge_connections_kind_check CHECK (kind IN ('github', 'gitlab', 'forgejo', 'gitea')),
    CONSTRAINT forge_connections_interval_check CHECK (interval_secs BETWEEN 60 AND 86400),
    CONSTRAINT forge_connections_credentials_check CHECK (num_nonnulls(credentials_enc, credentials_ref) = 1),
    CONSTRAINT forge_connections_webhook_mode_check CHECK (webhook_mode IN ('register', 'manual')),
    CONSTRAINT forge_connections_next_trigger_check CHECK (next_trigger IN ('schedule', 'manual', 'webhook')),
    CONSTRAINT forge_connections_webhook_check CHECK ((webhook_mode IS NULL) = (webhook_secret_enc IS NULL))
);

CREATE UNIQUE INDEX forge_connections_owner_key ON forge_connections (kind, api_url, lower(owner_path));
CREATE INDEX forge_connections_node_id_idx ON forge_connections (node_id, created_at);
CREATE INDEX forge_connections_next_run_at_idx ON forge_connections (next_run_at);
