ALTER TABLE knowledge_scans DROP CONSTRAINT knowledge_scans_source_check;
UPDATE knowledge_scans SET source = 'repo_sync' WHERE source = 'forge';
UPDATE knowledge_scans SET error_code = CASE WHEN error_code = 'forge.interrupted' THEN 'repo_sync.interrupted' ELSE regexp_replace(error_code, '^forge\.', 'hosting.') END WHERE error_code LIKE 'forge.%';
ALTER TABLE knowledge_scans ADD CONSTRAINT knowledge_scans_source_check CHECK (source IS NULL OR source IN ('repo_sync', 'remote', 'local_dir', 'local_git'));
