ALTER TABLE forge_sync_runs RENAME TO repo_sync_runs;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT forge_sync_runs_connection_id_fkey TO repo_sync_runs_connection_id_fkey;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT forge_sync_runs_error_code_check TO repo_sync_runs_error_code_check;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT forge_sync_runs_pkey TO repo_sync_runs_pkey;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT forge_sync_runs_problems_array_check TO repo_sync_runs_problems_array_check;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT forge_sync_runs_status_check TO repo_sync_runs_status_check;
ALTER TABLE repo_sync_runs RENAME CONSTRAINT forge_sync_runs_trigger_check TO repo_sync_runs_trigger_check;
ALTER INDEX forge_sync_runs_connection_id_idx RENAME TO repo_sync_runs_connection_id_idx;
UPDATE repo_sync_runs SET error_code = CASE WHEN error_code = 'forge.interrupted' THEN 'repo_sync.interrupted' ELSE regexp_replace(error_code, '^forge\.', 'hosting.') END WHERE error_code LIKE 'forge.%';
UPDATE repo_sync_runs
SET problems = (SELECT jsonb_agg(CASE WHEN p ? 'code' THEN jsonb_set(p, '{code}', to_jsonb(CASE WHEN (p->>'code') = 'forge.interrupted' THEN 'repo_sync.interrupted' ELSE regexp_replace((p->>'code'), '^forge\.', 'hosting.') END)) ELSE p END)
                FROM jsonb_array_elements(problems) p)
WHERE problems::text LIKE '%"forge.%';
