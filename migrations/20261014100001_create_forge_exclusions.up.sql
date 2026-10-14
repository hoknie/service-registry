CREATE TABLE forge_exclusions
(
    connection_id uuid        NOT NULL,
    external_id   text        NOT NULL,
    full_path     text        NOT NULL,
    project_id    uuid,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT forge_exclusions_pkey PRIMARY KEY (connection_id, external_id),
    CONSTRAINT forge_exclusions_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES forge_connections (id) ON DELETE CASCADE
);
