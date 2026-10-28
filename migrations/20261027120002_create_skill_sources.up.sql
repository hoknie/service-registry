CREATE TABLE skill_sources
(
    id                    uuid PRIMARY KEY,
    folder_id             uuid        NOT NULL,
    kind                  text        NOT NULL,
    forge                 text,
    url                   text,
    api_url               text,
    path                  text,
    working_tree          boolean     NOT NULL DEFAULT true,
    credentials_secret_id uuid,
    branch                text        NOT NULL DEFAULT '',
    root                  text        NOT NULL DEFAULT '',
    interval_secs         integer,
    new_status            text        NOT NULL DEFAULT 'published',
    versions              text        NOT NULL DEFAULT 'branch',
    tag_pattern           text        NOT NULL DEFAULT 'v{version}',
    next_run_at           timestamptz,
    lease_until           timestamptz,
    last_commit           text,
    last_tags             jsonb       NOT NULL DEFAULT '{}',
    last_run              jsonb,
    config_changed_at     timestamptz NOT NULL DEFAULT now(),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT skill_sources_folder_id_fkey FOREIGN KEY (folder_id) REFERENCES skill_folders (id) ON DELETE CASCADE,
    CONSTRAINT skill_sources_credentials_secret_id_fkey FOREIGN KEY (credentials_secret_id) REFERENCES secrets (id),
    CONSTRAINT skill_sources_kind_check CHECK (kind IN ('remote', 'local_dir', 'local_git')),
    CONSTRAINT skill_sources_forge_check CHECK (forge IN ('github', 'gitlab', 'gitea', 'forgejo')),
    CONSTRAINT skill_sources_url_check CHECK (char_length(url) <= 2048),
    CONSTRAINT skill_sources_api_url_check CHECK (char_length(api_url) <= 2048),
    CONSTRAINT skill_sources_path_check CHECK (char_length(path) <= 4096),
    CONSTRAINT skill_sources_fields_check CHECK (
        (kind = 'remote' AND forge IS NOT NULL AND url IS NOT NULL AND api_url IS NOT NULL AND path IS NULL)
            OR (kind <> 'remote' AND path IS NOT NULL AND forge IS NULL AND url IS NULL AND api_url IS NULL
                AND credentials_secret_id IS NULL)),
    CONSTRAINT skill_sources_branch_check CHECK (char_length(branch) <= 255),
    CONSTRAINT skill_sources_root_check CHECK (char_length(root) <= 512),
    CONSTRAINT skill_sources_interval_secs_check CHECK (interval_secs BETWEEN 300 AND 86400),
    CONSTRAINT skill_sources_new_status_check CHECK (new_status IN ('draft', 'published')),
    CONSTRAINT skill_sources_versions_check CHECK (versions IN ('branch', 'tags')),
    CONSTRAINT skill_sources_tag_pattern_check CHECK (char_length(tag_pattern) BETWEEN 1 AND 200),
    CONSTRAINT skill_sources_last_commit_check CHECK (char_length(last_commit) <= 200),
    CONSTRAINT skill_sources_last_tags_check CHECK (jsonb_typeof(last_tags) = 'object'),
    CONSTRAINT skill_sources_last_run_check CHECK (last_run IS NULL OR jsonb_typeof(last_run) = 'object')
);

CREATE INDEX skill_sources_folder_idx ON skill_sources (folder_id);

CREATE INDEX skill_sources_due_idx ON skill_sources (next_run_at) WHERE next_run_at IS NOT NULL;
