ALTER TABLE knowledge_sources
    ADD CONSTRAINT knowledge_sources_no_secret_check CHECK (credentials_secret_id IS NULL);

ALTER TABLE knowledge_sources
    DROP CONSTRAINT knowledge_sources_no_secret_check,
    DROP CONSTRAINT knowledge_sources_local_secret_check,
    DROP CONSTRAINT knowledge_sources_credentials_check,
    ADD CONSTRAINT knowledge_sources_credentials_check CHECK (credentials_enc IS NULL OR credentials_ref IS NULL),
    DROP COLUMN credentials_secret_id;
