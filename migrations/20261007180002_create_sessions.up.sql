CREATE TABLE sessions
(
    id           uuid PRIMARY KEY,
    user_id      uuid        NOT NULL,
    token_hash   bytea       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT sessions_token_hash_key UNIQUE (token_hash),
    CONSTRAINT sessions_token_hash_len_check CHECK (octet_length(token_hash) = 32)
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
