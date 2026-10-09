CREATE TABLE role_bindings
(
    id         uuid PRIMARY KEY,
    node_id    uuid        NOT NULL,
    user_id    uuid,
    group_id   uuid,
    role       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT role_bindings_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT role_bindings_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT role_bindings_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
    CONSTRAINT role_bindings_subject_check CHECK (num_nonnulls(user_id, group_id) = 1),
    CONSTRAINT role_bindings_role_check CHECK (role IN ('viewer', 'editor', 'admin'))
);

CREATE UNIQUE INDEX role_bindings_subject_key ON role_bindings (node_id, user_id, group_id) NULLS NOT DISTINCT;
CREATE INDEX role_bindings_user_id_idx ON role_bindings (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX role_bindings_group_id_idx ON role_bindings (group_id) WHERE group_id IS NOT NULL;
