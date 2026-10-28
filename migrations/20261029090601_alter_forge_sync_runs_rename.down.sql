UPDATE repo_sync_runs SET error_code = CASE WHEN error_code = 'repo_sync.interrupted' THEN 'forge.interrupted' ELSE regexp_replace(error_code, '^hosting\.', 'forge.') END WHERE error_code LIKE 'hosting.%' OR error_code = 'repo_sync.interrupted';
UPDATE repo_sync_runs
SET problems = (SELECT jsonb_agg(CASE WHEN p ? 'code' THEN jsonb_set(p, '{code}', to_jsonb(CASE WHEN (p->>'code') = 'repo_sync.interrupted' THEN 'forge.interrupted' ELSE regexp_replace((p->>'code'), '^hosting\.', 'forge.') END)) ELSE p END)
                FROM jsonb_array_elements(problems) p)
WHERE problems::text ~ '"(hosting\.|repo_sync\.interrupted)';
ALTER TABLE repo_sync_runs RENAME CONSTRAINT repo_sync_runs_connection_id_fkey TO forge_sync_runs_connection_id_fkey;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT repo_sync_runs_error_code_check TO forge_sync_runs_error_code_check;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT repo_sync_runs_pkey TO forge_sync_runs_pkey;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT repo_sync_runs_problems_array_check TO forge_sync_runs_problems_array_check;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT repo_sync_runs_status_check TO forge_sync_runs_status_check;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT repo_sync_runs_trigger_check TO forge_sync_runs_trigger_check;
ALTER INDEX repo_sync_runs_connection_id_idx RENAME TO forge_sync_runs_connection_id_idx;
ALTER TABLE repo_sync_runs RENAME TO forge_sync_runs;
