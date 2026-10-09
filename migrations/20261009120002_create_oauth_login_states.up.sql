CREATE TABLE oauth_login_states
(
    id            uuid PRIMARY KEY,
    browser_hash  bytea       NOT NULL,
    provider      text        NOT NULL,
    state         text        NOT NULL,
    nonce         text        NOT NULL,
    code_verifier text        NOT NULL,
    next          text,
    link_user_id  uuid,
    expires_at    timestamptz NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT oauth_login_states_state_key UNIQUE (state),
    CONSTRAINT oauth_login_states_browser_hash_len_check CHECK (octet_length(browser_hash) = 32),
    CONSTRAINT oauth_login_states_link_user_id_fkey FOREIGN KEY (link_user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX oauth_login_states_expires_idx ON oauth_login_states (expires_at);
