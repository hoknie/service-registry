CREATE TABLE knowledge_patterns
(
    node_id     uuid PRIMARY KEY,
    include_set boolean     NOT NULL DEFAULT false,
    include     text[],
    exclude     text[],
    branches    text[],
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT knowledge_patterns_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT knowledge_patterns_include_check CHECK (include_set OR include IS NULL)
);

INSERT INTO knowledge_patterns (node_id, include_set, include, exclude, branches)
SELECT project_id,
       include IS NOT NULL,
       include,
       CASE WHEN exclude = '{}' THEN NULL ELSE exclude END,
       CASE WHEN branches = '{}' THEN NULL ELSE branches END
FROM knowledge_settings
WHERE include IS NOT NULL
   OR exclude <> '{}'
   OR branches <> '{}';

ALTER TABLE knowledge_settings DROP COLUMN include, DROP COLUMN exclude, DROP COLUMN branches;
