ALTER TABLE forge_connections
    DROP CONSTRAINT forge_connections_credentials_check,
    ADD CONSTRAINT forge_connections_credentials_check CHECK (num_nonnulls(credentials_enc, credentials_ref) = 1),
    DROP COLUMN credentials_secret_id;
