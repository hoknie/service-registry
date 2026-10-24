CREATE TABLE node_owner_clears
(
    node_id    uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT node_owner_clears_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE
);
