CREATE TABLE skill_folders
(
    id              uuid PRIMARY KEY,
    parent_id       uuid,
    organization_id uuid,
    slug            text        NOT NULL,
    name            text        NOT NULL,
    description     text        NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT skill_folders_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES skill_folders (id) ON DELETE CASCADE,
    CONSTRAINT skill_folders_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT skill_folders_slug_check CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    CONSTRAINT skill_folders_name_check CHECK (char_length(name) BETWEEN 1 AND 200),
    CONSTRAINT skill_folders_description_check CHECK (char_length(description) <= 2000)
);

CREATE UNIQUE INDEX skill_folders_parent_slug_idx ON skill_folders (parent_id, slug) WHERE parent_id IS NOT NULL;

CREATE UNIQUE INDEX skill_folders_organization_slug_idx ON skill_folders (organization_id, slug)
    WHERE parent_id IS NULL AND organization_id IS NOT NULL;

CREATE UNIQUE INDEX skill_folders_global_slug_idx ON skill_folders (slug) WHERE parent_id IS NULL AND organization_id IS NULL;

CREATE INDEX skill_folders_organization_idx ON skill_folders (organization_id);
