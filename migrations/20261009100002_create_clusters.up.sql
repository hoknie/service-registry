CREATE TABLE clusters
(
    id                      uuid PRIMARY KEY,
    name                    text        NOT NULL,
    environment             text        NOT NULL,
    in_cluster              boolean     NOT NULL DEFAULT false,
    api_url                 text,
    ca_pem                  text,
    credentials_enc         text,
    credentials_ref         text,
    credentials_fingerprint text,
    namespaces              text[]      NOT NULL DEFAULT '{}',
    rules                   jsonb       NOT NULL DEFAULT '[]'::jsonb,
    interval_secs           integer     NOT NULL DEFAULT 60,
    enabled                 boolean     NOT NULL DEFAULT true,
    status                  text        NOT NULL DEFAULT 'never',
    last_error              jsonb,
    last_polled_at          timestamptz,
    next_run_at             timestamptz NOT NULL DEFAULT now(),
    lease_until             timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT clusters_status_check CHECK (status IN ('ok', 'error', 'never')),
    CONSTRAINT clusters_interval_check CHECK (interval_secs BETWEEN 15 AND 3600),
    CONSTRAINT clusters_rules_check CHECK (jsonb_typeof(rules) = 'array'),
    CONSTRAINT clusters_credentials_check CHECK (
        (in_cluster AND api_url IS NULL AND credentials_enc IS NULL AND credentials_ref IS NULL)
            OR (NOT in_cluster AND api_url IS NOT NULL AND (credentials_enc IS NULL) <> (credentials_ref IS NULL))
        )
);

CREATE UNIQUE INDEX clusters_name_key ON clusters (lower(name));
CREATE INDEX clusters_next_run_idx ON clusters (next_run_at);
