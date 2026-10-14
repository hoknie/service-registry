CREATE TABLE knowledge_source_repos
(
    project_id        uuid PRIMARY KEY,
    full_path         text        NOT NULL,
    web_url           text        NOT NULL,
    description       text        NOT NULL DEFAULT '',
    topics            text[]      NOT NULL DEFAULT '{}',
    readme_md         text,
    readme_truncated  boolean     NOT NULL DEFAULT false,
    languages         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    default_branch    text,
    archived          boolean     NOT NULL DEFAULT false,
    visibility        text        NOT NULL,
    stars             integer     NOT NULL DEFAULT 0,
    license           text,
    pushed_at         timestamptz,
    source_updated_at timestamptz,
    synced_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT knowledge_source_repos_project_id_fkey FOREIGN KEY (project_id) REFERENCES knowledge_sources (project_id) ON DELETE CASCADE,
    CONSTRAINT knowledge_source_repos_visibility_check CHECK (visibility IN ('public', 'internal', 'private')),
    CONSTRAINT knowledge_source_repos_languages_object_check CHECK (jsonb_typeof(languages) = 'object')
);
