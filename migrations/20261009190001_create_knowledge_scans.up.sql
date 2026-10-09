CREATE TABLE knowledge_scans
(
    id           uuid PRIMARY KEY,
    project_id   uuid        NOT NULL,
    kind         text        NOT NULL,
    trigger      text        NOT NULL,
    source       text,
    status       text        NOT NULL,
    started_at   timestamptz NOT NULL,
    finished_at  timestamptz NOT NULL,
    repeats      integer     NOT NULL DEFAULT 0,
    branches     jsonb       NOT NULL DEFAULT '[]'::jsonb,
    index        jsonb,
    error_code   text,
    error_detail text,
    warnings     text[]      NOT NULL DEFAULT '{}',
    CONSTRAINT knowledge_scans_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT knowledge_scans_kind_check CHECK (kind IN ('collect', 'index')),
    CONSTRAINT knowledge_scans_trigger_check CHECK (trigger IN ('schedule', 'manual')),
    CONSTRAINT knowledge_scans_source_check CHECK (source IS NULL OR source IN ('forge', 'remote', 'local_dir', 'local_git')),
    CONSTRAINT knowledge_scans_status_check CHECK (status IN ('ok', 'unchanged', 'warning', 'failed')),
    CONSTRAINT knowledge_scans_repeats_check CHECK (repeats >= 0),
    CONSTRAINT knowledge_scans_error_detail_check CHECK (char_length(error_detail) <= 1000)
);

CREATE INDEX knowledge_scans_project_idx ON knowledge_scans (project_id, kind, started_at DESC);
CREATE INDEX knowledge_scans_started_idx ON knowledge_scans (started_at DESC);
