ALTER TABLE skill_sources
    ADD COLUMN roots       text[] NOT NULL DEFAULT '{""}',
    ADD COLUMN layout      text   NOT NULL DEFAULT 'flat',
    ADD COLUMN group_field text;

UPDATE skill_sources
SET roots = ARRAY [root];

ALTER TABLE skill_sources
    DROP COLUMN root,
    DROP CONSTRAINT skill_sources_new_status_check,
    ADD CONSTRAINT skill_sources_new_status_check CHECK (new_status IN ('draft', 'published', 'disabled')),
    ADD CONSTRAINT skill_sources_roots_check CHECK (cardinality(roots) BETWEEN 1 AND 20
        AND array_position(roots, NULL) IS NULL
        AND char_length(array_to_string(roots, '')) <= 10240),
    ADD CONSTRAINT skill_sources_layout_check CHECK (layout IN ('flat', 'tree', 'field')),
    ADD CONSTRAINT skill_sources_group_field_check CHECK (
        (layout = 'field') = (group_field IS NOT NULL) AND char_length(group_field) BETWEEN 1 AND 100);
