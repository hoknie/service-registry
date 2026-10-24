CREATE TABLE group_permissions (
    group_id uuid   NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    section  text   NOT NULL,
    actions  text[] NOT NULL,
    PRIMARY KEY (group_id, section),
    CONSTRAINT group_permissions_section_check CHECK (section IN ('users', 'groups', 'tokens', 'clusters', 'environments', 'link_kinds', 'secrets', 'scans')),
    CONSTRAINT group_permissions_actions_check CHECK (cardinality(actions) BETWEEN 1 AND 4 AND actions <@ ARRAY['create', 'read', 'update', 'delete']::text[])
);
