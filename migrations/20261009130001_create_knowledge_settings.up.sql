CREATE TABLE knowledge_settings
(
    project_id  uuid PRIMARY KEY,
    include     text[],
    exclude     text[]      NOT NULL DEFAULT '{}',
    branches    text[]      NOT NULL DEFAULT '{}',
    force       boolean     NOT NULL DEFAULT false,
    next_run_at timestamptz NOT NULL DEFAULT now(),
    lease_until timestamptz,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT knowledge_settings_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE
);

CREATE INDEX knowledge_settings_next_run_at_idx ON knowledge_settings (next_run_at);
