UPDATE skill_sources
SET new_status = 'draft'
WHERE new_status = 'disabled';

ALTER TABLE skill_sources
    ADD COLUMN root text NOT NULL DEFAULT '';

UPDATE skill_sources
SET root = left(roots[1], 512);

ALTER TABLE skill_sources
    DROP CONSTRAINT skill_sources_new_status_check,
    DROP CONSTRAINT skill_sources_roots_check,
    DROP CONSTRAINT skill_sources_layout_check,
    DROP CONSTRAINT skill_sources_group_field_check,
    DROP COLUMN roots,
    DROP COLUMN layout,
    DROP COLUMN group_field,
    ADD CONSTRAINT skill_sources_new_status_check CHECK (new_status IN ('draft', 'published')),
    ADD CONSTRAINT skill_sources_root_check CHECK (char_length(root) <= 512);
