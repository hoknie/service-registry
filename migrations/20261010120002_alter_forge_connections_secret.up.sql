ALTER TABLE forge_connections
    ADD COLUMN credentials_secret_id uuid,
    ADD CONSTRAINT forge_connections_credentials_secret_id_fkey FOREIGN KEY (credentials_secret_id) REFERENCES secrets (id),
    DROP CONSTRAINT forge_connections_credentials_check,
    ADD CONSTRAINT forge_connections_credentials_check CHECK (num_nonnulls(credentials_enc, credentials_ref, credentials_secret_id) = 1);
