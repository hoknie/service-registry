CREATE TABLE knowledge_sources
(
    project_id              uuid PRIMARY KEY,
    kind                    text        NOT NULL,
    forge                   text,
    url                     text,
    api_url                 text,
    path                    text,
    credentials_enc         text,
    credentials_ref         text,
    credentials_fingerprint text,
    heads                   jsonb       NOT NULL DEFAULT '{}',
    default_branch          text,
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT knowledge_sources_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT knowledge_sources_kind_check CHECK (kind IN ('remote', 'local_dir', 'local_git')),
    CONSTRAINT knowledge_sources_forge_check CHECK (forge IN ('github', 'gitlab', 'gitea', 'forgejo')),
    CONSTRAINT knowledge_sources_fields_check CHECK (
        (kind = 'remote' AND forge IS NOT NULL AND url IS NOT NULL AND api_url IS NOT NULL AND path IS NULL)
            OR (kind <> 'remote' AND path IS NOT NULL AND forge IS NULL AND url IS NULL AND api_url IS NULL
            AND credentials_enc IS NULL AND credentials_ref IS NULL)),
    CONSTRAINT knowledge_sources_credentials_check CHECK (credentials_enc IS NULL OR credentials_ref IS NULL),
    CONSTRAINT knowledge_sources_heads_check CHECK (jsonb_typeof(heads) = 'object')
);
