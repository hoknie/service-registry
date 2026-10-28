CREATE TABLE skills
(
    id              uuid PRIMARY KEY,
    folder_id       uuid,
    organization_id uuid,
    slug            text        NOT NULL,
    name            text        NOT NULL,
    title           text        NOT NULL,
    description     text        NOT NULL,
    license         text        NOT NULL DEFAULT '',
    compatibility   text        NOT NULL DEFAULT '',
    allowed_tools   text[]      NOT NULL DEFAULT '{}',
    tags            text[]      NOT NULL DEFAULT '{}',
    author          text        NOT NULL DEFAULT '',
    image_path      text,
    status          text        NOT NULL DEFAULT 'draft',
    image_sha256    bytea,
    image_type      text,
    downloads       bigint      NOT NULL DEFAULT 0,
    source_id       uuid,
    source_path     text,
    created_by      uuid,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT skills_folder_id_fkey FOREIGN KEY (folder_id) REFERENCES skill_folders (id) ON DELETE CASCADE,
    CONSTRAINT skills_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT skills_source_id_fkey FOREIGN KEY (source_id) REFERENCES skill_sources (id) ON DELETE CASCADE,
    CONSTRAINT skills_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT skills_slug_check CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    CONSTRAINT skills_name_check CHECK (name ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(name) <= 64),
    CONSTRAINT skills_title_check CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT skills_description_check CHECK (char_length(description) BETWEEN 1 AND 1024),
    CONSTRAINT skills_license_check CHECK (char_length(license) <= 200),
    CONSTRAINT skills_compatibility_check CHECK (char_length(compatibility) <= 500),
    CONSTRAINT skills_allowed_tools_check CHECK (cardinality(allowed_tools) <= 100),
    CONSTRAINT skills_tags_check CHECK (cardinality(tags) <= 20),
    CONSTRAINT skills_author_check CHECK (char_length(author) <= 200),
    CONSTRAINT skills_image_path_check CHECK (char_length(image_path) <= 512),
    CONSTRAINT skills_status_check CHECK (status IN ('draft', 'published')),
    CONSTRAINT skills_image_sha256_check CHECK (octet_length(image_sha256) = 32),
    CONSTRAINT skills_image_type_check CHECK (image_type IN ('image/png', 'image/jpeg', 'image/webp', 'image/svg+xml')),
    CONSTRAINT skills_image_check CHECK ((image_sha256 IS NULL) = (image_type IS NULL)),
    CONSTRAINT skills_downloads_check CHECK (downloads >= 0),
    CONSTRAINT skills_source_path_check CHECK (char_length(source_path) <= 512),
    CONSTRAINT skills_source_check CHECK ((source_id IS NULL) = (source_path IS NULL))
);

CREATE UNIQUE INDEX skills_folder_slug_idx ON skills (folder_id, slug) WHERE folder_id IS NOT NULL;

CREATE UNIQUE INDEX skills_organization_slug_idx ON skills (organization_id, slug)
    WHERE folder_id IS NULL AND organization_id IS NOT NULL;

CREATE UNIQUE INDEX skills_global_slug_idx ON skills (slug) WHERE folder_id IS NULL AND organization_id IS NULL;

CREATE UNIQUE INDEX skills_source_path_idx ON skills (source_id, source_path) WHERE source_id IS NOT NULL;

CREATE INDEX skills_organization_idx ON skills (organization_id);

CREATE INDEX skills_tags_idx ON skills USING gin (tags);
