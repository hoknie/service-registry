UPDATE knowledge_settings
SET next_run_at = now()
WHERE next_run_at IS NULL;

ALTER TABLE knowledge_settings
    DROP COLUMN triggered_at,
    ALTER COLUMN next_run_at SET NOT NULL;
