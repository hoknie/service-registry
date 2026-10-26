ALTER TABLE knowledge_snapshots
    ADD COLUMN ref_kind text NOT NULL DEFAULT 'branch',
    ADD CONSTRAINT knowledge_snapshots_ref_kind_check CHECK (ref_kind IN ('branch', 'tag'));

DROP INDEX knowledge_snapshots_branch_idx;

CREATE INDEX knowledge_snapshots_ref_idx ON knowledge_snapshots (project_id, ref_kind, branch, collected_at DESC);
