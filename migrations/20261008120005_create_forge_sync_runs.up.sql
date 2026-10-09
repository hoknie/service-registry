CREATE TABLE forge_sync_runs
(
    id            uuid PRIMARY KEY,
    connection_id uuid        NOT NULL,
    trigger       text        NOT NULL,
    status        text        NOT NULL,
    started_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,
    created       integer     NOT NULL DEFAULT 0,
    updated       integer     NOT NULL DEFAULT 0,
    orphaned      integer     NOT NULL DEFAULT 0,
    skipped       integer     NOT NULL DEFAULT 0,
    error_code    text,
    error_message text,
    problems      jsonb       NOT NULL DEFAULT '[]'::jsonb,
    CONSTRAINT forge_sync_runs_connection_id_fkey FOREIGN KEY (connection_id) REFERENCES forge_connections (id) ON DELETE CASCADE,
    CONSTRAINT forge_sync_runs_trigger_check CHECK (trigger IN ('schedule', 'manual', 'webhook', 'cli')),
    CONSTRAINT forge_sync_runs_status_check CHECK (status IN ('running', 'succeeded', 'failed', 'rate_limited')),
    CONSTRAINT forge_sync_runs_problems_array_check CHECK (jsonb_typeof(problems) = 'array')
);

CREATE INDEX forge_sync_runs_connection_id_idx ON forge_sync_runs (connection_id, started_at);
