ALTER TABLE knowledge_settings
    ADD COLUMN include text[],
    ADD COLUMN exclude  text[] NOT NULL DEFAULT '{}',
    ADD COLUMN branches text[] NOT NULL DEFAULT '{}';

INSERT INTO knowledge_settings (project_id, include, exclude, branches)
SELECT p.node_id, p.include, COALESCE(p.exclude, '{}'), COALESCE(p.branches, '{}')
FROM knowledge_patterns p
         JOIN nodes n ON n.id = p.node_id AND n.kind = 'project' ON CONFLICT (project_id) DO
UPDATE SET include = EXCLUDED.include, exclude = EXCLUDED.exclude, branches = EXCLUDED.branches;

DROP TABLE knowledge_patterns;
