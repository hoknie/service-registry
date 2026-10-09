CREATE TABLE user_identities
(
    id            uuid PRIMARY KEY,
    user_id       uuid        NOT NULL,
    provider      text        NOT NULL,
    subject       text        NOT NULL,
    email         text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT user_identities_subject_key UNIQUE (provider, subject)
);

CREATE INDEX user_identities_user_id_idx ON user_identities (user_id);
