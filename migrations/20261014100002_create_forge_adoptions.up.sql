CREATE TABLE forge_adoptions
(
    connection_id uuid        NOT NULL,
    external_id   text        NOT NULL,
    project_id    uuid        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT forge_adoptions_pkey PRIMARY KEY (connection_id, external_id),
    CONSTRAINT forge_adoptions_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES forge_connections (id) ON DELETE CASCADE
);
