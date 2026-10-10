ALTER TABLE knowledge_sources
    ADD COLUMN credentials_secret_id uuid,
    ADD CONSTRAINT knowledge_sources_credentials_secret_id_fkey FOREIGN KEY (credentials_secret_id) REFERENCES secrets (id),
    DROP CONSTRAINT knowledge_sources_credentials_check,
    ADD CONSTRAINT knowledge_sources_credentials_check CHECK (num_nonnulls(credentials_enc, credentials_ref, credentials_secret_id) <= 1),
    ADD CONSTRAINT knowledge_sources_local_secret_check CHECK (kind = 'remote' OR credentials_secret_id IS NULL);
