CREATE TABLE knowledge_embeddings
(
    sha256      bytea       NOT NULL,
    model       text        NOT NULL,
    ord         integer     NOT NULL,
    chunk_start integer     NOT NULL,
    chunk_end   integer     NOT NULL,
    vector      real[]      NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT knowledge_embeddings_pkey PRIMARY KEY (sha256, model, ord),
    CONSTRAINT knowledge_embeddings_sha256_fkey FOREIGN KEY (sha256) REFERENCES knowledge_blobs (sha256) ON DELETE CASCADE,
    CONSTRAINT knowledge_embeddings_range_check CHECK (chunk_start >= 0 AND chunk_end > chunk_start)
);

CREATE TABLE knowledge_index_state
(
    project_id  uuid PRIMARY KEY,
    next_run_at timestamptz NOT NULL DEFAULT now(),
    lease_until timestamptz,
    synced_at   timestamptz,
    engine      text,
    model       text,
    fingerprint text,
    failure     text,
    CONSTRAINT knowledge_index_state_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE
);

CREATE INDEX knowledge_index_state_next_run_at_idx ON knowledge_index_state (next_run_at);

DO
$$
BEGIN
    IF
EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'vector')
       AND NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
BEGIN
            CREATE
EXTENSION vector SCHEMA public;
EXCEPTION WHEN unique_violation OR duplicate_object THEN
            NULL;
END;
END IF;
END
$$;
