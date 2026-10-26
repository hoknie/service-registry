ALTER TABLE knowledge_patterns
    ADD COLUMN refs jsonb,
    ADD CONSTRAINT knowledge_patterns_refs_check
        CHECK (refs IS NULL OR (jsonb_typeof(refs) = 'array' AND jsonb_array_length(refs) <= 50));

UPDATE knowledge_patterns
SET refs = (SELECT jsonb_agg(jsonb_build_object('pattern', p, 'kind', 'branch', 'release', false) ORDER BY i)
            FROM unnest(branches) WITH ORDINALITY AS u(p, i))
WHERE branches IS NOT NULL;

UPDATE knowledge_patterns SET refs = '[]'::jsonb WHERE branches IS NOT NULL AND refs IS NULL;

ALTER TABLE knowledge_patterns DROP COLUMN branches;
