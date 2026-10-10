DROP INDEX knowledge_scans_project_idx;
DROP INDEX knowledge_scans_last_started_idx;

ALTER TABLE knowledge_scans
    DROP COLUMN last_started_at,
    DROP COLUMN duration_ms;

CREATE INDEX knowledge_scans_project_idx ON knowledge_scans (project_id, kind, started_at DESC);
CREATE INDEX knowledge_scans_started_idx ON knowledge_scans (started_at DESC);
