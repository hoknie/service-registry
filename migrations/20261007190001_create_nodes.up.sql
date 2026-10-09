CREATE TABLE nodes
(
    id             uuid PRIMARY KEY,
    kind           text        NOT NULL,
    parent_id      uuid,
    slug           text        NOT NULL,
    name           text        NOT NULL,
    description    text        NOT NULL DEFAULT '',
    labels         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    forge          text,
    repo_url       text,
    default_branch text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT nodes_kind_check CHECK (kind IN ('organization', 'folder', 'project')),
    CONSTRAINT nodes_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES nodes (id) ON DELETE RESTRICT,
    CONSTRAINT nodes_not_own_parent_check CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT nodes_labels_object_check CHECK (jsonb_typeof(labels) = 'object'),
    CONSTRAINT nodes_forge_check CHECK (forge IN ('github', 'gitlab', 'forgejo')),
    CONSTRAINT nodes_repo_fields_check CHECK (
        kind = 'project' OR (forge IS NULL AND repo_url IS NULL AND default_branch IS NULL)
        )
);

CREATE UNIQUE INDEX nodes_parent_slug_key ON nodes (parent_id, slug) NULLS NOT DISTINCT;
