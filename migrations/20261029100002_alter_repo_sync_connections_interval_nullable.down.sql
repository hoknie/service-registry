UPDATE repo_sync_connections
SET interval_secs = 900
WHERE interval_secs IS NULL;

ALTER TABLE repo_sync_connections
    ALTER COLUMN interval_secs SET NOT NULL;
