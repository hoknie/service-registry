DELETE FROM knowledge_patterns
WHERE (api_paths IS NOT NULL OR refresh_events IS NOT NULL)
  AND NOT include_set
  AND exclude IS NULL
  AND branches IS NULL
  AND apis IS NULL
  AND refresh IS NULL;

ALTER TABLE knowledge_patterns
    DROP COLUMN refresh_events,
    DROP COLUMN api_paths;
