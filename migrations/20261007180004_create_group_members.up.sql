CREATE TABLE group_members
(
    id         uuid PRIMARY KEY,
    group_id   uuid        NOT NULL,
    user_id    uuid        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT group_members_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
    CONSTRAINT group_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT group_members_group_user_key UNIQUE (group_id, user_id)
);

CREATE INDEX group_members_user_id_idx ON group_members (user_id);
