ALTER TABLE knowledge_patterns ADD COLUMN branches text[];

UPDATE knowledge_patterns
SET branches = ARRAY(SELECT e ->> 'pattern'
                     FROM jsonb_array_elements(refs) WITH ORDINALITY AS u(e, i)
                     WHERE e ->> 'kind' = 'branch'
                     ORDER BY i)
WHERE refs IS NOT NULL;

ALTER TABLE knowledge_patterns DROP COLUMN refs;
