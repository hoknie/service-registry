CREATE TABLE docs_publications
(
    node_id         uuid PRIMARY KEY,
    publish         boolean,
    versions        jsonb,
    default_branch  boolean,
    current_release boolean,
    name            text,
    host            text,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT docs_publications_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT docs_publications_versions_check
        CHECK (versions IS NULL OR (jsonb_typeof(versions) = 'array' AND jsonb_array_length(versions) <= 50)),
    CONSTRAINT docs_publications_name_check CHECK (name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    CONSTRAINT docs_publications_host_check CHECK (char_length(host) BETWEEN 1 AND 253 AND host = lower(host))
);

CREATE UNIQUE INDEX docs_publications_name_idx ON docs_publications (name) WHERE name IS NOT NULL;

CREATE UNIQUE INDEX docs_publications_host_idx ON docs_publications (host) WHERE host IS NOT NULL;
