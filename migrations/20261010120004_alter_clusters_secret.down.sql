ALTER TABLE clusters
    DROP CONSTRAINT clusters_credentials_check,
    ADD CONSTRAINT clusters_credentials_check CHECK (
        (in_cluster AND api_url IS NULL AND credentials_enc IS NULL AND credentials_ref IS NULL)
            OR (NOT in_cluster AND api_url IS NOT NULL AND (credentials_enc IS NULL) <> (credentials_ref IS NULL))
        ),
    DROP COLUMN credentials_secret_id;
