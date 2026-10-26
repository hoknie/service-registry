DELETE FROM knowledge_snapshots WHERE ref_kind = 'tag';

DROP INDEX knowledge_snapshots_ref_idx;

CREATE INDEX knowledge_snapshots_branch_idx ON knowledge_snapshots (project_id, branch, collected_at DESC);

ALTER TABLE knowledge_snapshots DROP COLUMN ref_kind;
