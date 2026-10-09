CREATE TABLE project_keys
(
    id           uuid PRIMARY KEY,
    project_id   uuid        NOT NULL,
    prefix       text        NOT NULL,
    key_hash     bytea       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    expires_at   timestamptz,
    revoked_at   timestamptz,
    CONSTRAINT project_keys_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT project_keys_key_hash_key UNIQUE (key_hash),
    CONSTRAINT project_keys_key_hash_len_check CHECK (octet_length(key_hash) = 32)
);

CREATE INDEX project_keys_project_id_idx ON project_keys (project_id, created_at);
