CREATE TABLE secrets
(
    id          uuid PRIMARY KEY,
    node_id     uuid,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    value_enc   text,
    value_ref   text,
    fingerprint text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT secrets_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT secrets_name_check CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT secrets_description_check CHECK (char_length(description) <= 500),
    CONSTRAINT secrets_value_check CHECK (num_nonnulls(value_enc, value_ref) = 1)
);

CREATE UNIQUE INDEX secrets_name_key ON secrets (COALESCE(node_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name));
