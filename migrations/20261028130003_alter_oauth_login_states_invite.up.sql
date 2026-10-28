ALTER TABLE oauth_login_states
    ADD COLUMN invite_hash bytea,
    ADD CONSTRAINT oauth_login_states_invite_hash_len_check CHECK (octet_length(invite_hash) = 32);
