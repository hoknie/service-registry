CREATE TABLE personal_access_tokens
(
    id           uuid PRIMARY KEY,
    user_id      uuid        NOT NULL,
    name         text        NOT NULL,
    prefix       text        NOT NULL,
    token_hash   bytea       NOT NULL,
    scopes       text[]      NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz,
    last_used_at timestamptz,
    revoked_at   timestamptz,
    CONSTRAINT personal_access_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT personal_access_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT personal_access_tokens_token_hash_len_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT personal_access_tokens_name_check CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT personal_access_tokens_scopes_check
        CHECK (cardinality(scopes) > 0 AND scopes <@ ARRAY['read', 'write', 'admin', 'mcp']::text[])
    );

CREATE INDEX personal_access_tokens_user_id_idx ON personal_access_tokens (user_id, created_at);
CREATE INDEX personal_access_tokens_prefix_idx ON personal_access_tokens (prefix text_pattern_ops);
