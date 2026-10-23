DELETE FROM knowledge_patterns
WHERE refresh IS NOT NULL
  AND NOT include_set
  AND exclude IS NULL
  AND branches IS NULL
  AND apis IS NULL;

ALTER TABLE knowledge_patterns
    DROP COLUMN refresh;
