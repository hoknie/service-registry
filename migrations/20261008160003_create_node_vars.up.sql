CREATE TABLE node_vars
(
    id         uuid PRIMARY KEY,
    node_id    uuid        NOT NULL,
    key        text        NOT NULL,
    value      text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT node_vars_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT node_vars_key_key UNIQUE (node_id, key)
);
