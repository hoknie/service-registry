ALTER TABLE knowledge_scans DROP CONSTRAINT knowledge_scans_source_check;
UPDATE knowledge_scans SET source = 'forge' WHERE source = 'repo_sync';
UPDATE knowledge_scans SET error_code = CASE WHEN error_code = 'repo_sync.interrupted' THEN 'forge.interrupted' ELSE regexp_replace(error_code, '^hosting\.', 'forge.') END WHERE error_code LIKE 'hosting.%' OR error_code = 'repo_sync.interrupted';
ALTER TABLE knowledge_scans ADD CONSTRAINT knowledge_scans_source_check CHECK (source IS NULL OR source IN ('forge', 'remote', 'local_dir', 'local_git'));
