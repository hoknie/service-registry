ALTER TABLE knowledge_scans
    ADD COLUMN last_started_at timestamptz,
    ADD COLUMN duration_ms integer;

UPDATE knowledge_scans
SET last_started_at = CASE WHEN repeats = 0 THEN started_at ELSE finished_at END,
    duration_ms = CASE WHEN repeats = 0 THEN round(extract(epoch FROM finished_at - started_at) * 1000)::integer END;

ALTER TABLE knowledge_scans
    ALTER COLUMN last_started_at SET NOT NULL,
    ADD CONSTRAINT knowledge_scans_duration_check CHECK (duration_ms IS NULL OR duration_ms >= 0);

DROP INDEX knowledge_scans_project_idx;
DROP INDEX knowledge_scans_started_idx;
CREATE INDEX knowledge_scans_project_idx ON knowledge_scans (project_id, kind, last_started_at DESC);
CREATE INDEX knowledge_scans_last_started_idx ON knowledge_scans (last_started_at DESC);
