CREATE TABLE knowledge_snapshots
(
    id           uuid PRIMARY KEY,
    project_id   uuid        NOT NULL,
    branch       text        NOT NULL,
    commit_sha   text        NOT NULL,
    status       text        NOT NULL,
    error_code   text,
    files        integer     NOT NULL DEFAULT 0,
    bytes        bigint      NOT NULL DEFAULT 0,
    skipped      integer     NOT NULL DEFAULT 0,
    truncated    boolean     NOT NULL DEFAULT false,
    collected_at timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT knowledge_snapshots_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT knowledge_snapshots_status_check CHECK (status IN ('ok', 'partial', 'failed'))
);

CREATE INDEX knowledge_snapshots_branch_idx ON knowledge_snapshots (project_id, branch, collected_at DESC);
