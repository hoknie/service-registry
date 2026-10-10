ALTER TABLE clusters
    ADD COLUMN credentials_secret_id uuid,
    ADD CONSTRAINT clusters_credentials_secret_id_fkey FOREIGN KEY (credentials_secret_id) REFERENCES secrets (id),
    DROP CONSTRAINT clusters_credentials_check,
    ADD CONSTRAINT clusters_credentials_check CHECK (
        (in_cluster AND api_url IS NULL AND credentials_enc IS NULL AND credentials_ref IS NULL AND credentials_secret_id IS NULL)
            OR (NOT in_cluster AND api_url IS NOT NULL AND num_nonnulls(credentials_enc, credentials_ref, credentials_secret_id) = 1)
        );
