CREATE TABLE organization_invitations
(
    id              uuid PRIMARY KEY,
    organization_id uuid        NOT NULL,
    email           text        NOT NULL,
    role            text        NOT NULL,
    locale          text        NOT NULL,
    token_hash      bytea       NOT NULL,
    invited_by      uuid,
    accepted_by     uuid,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    accepted_at     timestamptz,
    revoked_at      timestamptz,
    CONSTRAINT organization_invitations_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT organization_invitations_invited_by_fkey FOREIGN KEY (invited_by) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT organization_invitations_accepted_by_fkey FOREIGN KEY (accepted_by) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT organization_invitations_token_hash_key UNIQUE (token_hash),
    CONSTRAINT organization_invitations_token_hash_len_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT organization_invitations_email_check CHECK (char_length(email) BETWEEN 3 AND 254 AND email = lower(email)
        AND email ~ '^[^@[:space:]]+@[^@[:space:]]+$'),
    CONSTRAINT organization_invitations_role_check CHECK (role IN ('viewer', 'editor', 'admin')),
    CONSTRAINT organization_invitations_locale_check CHECK (locale IN ('en', 'es', 'ru', 'zh'))
);

CREATE UNIQUE INDEX organization_invitations_pending_idx ON organization_invitations (organization_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;
