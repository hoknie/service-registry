CREATE TABLE forge_groups
(
    node_id       uuid PRIMARY KEY,
    connection_id uuid        NOT NULL,
    full_path     text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT forge_groups_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT forge_groups_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES forge_connections (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX forge_groups_path_key ON forge_groups (connection_id, lower(full_path));
