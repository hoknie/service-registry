UPDATE skills
SET status = 'draft'
WHERE status = 'disabled';

ALTER TABLE skills
    DROP CONSTRAINT skills_status_check,
    ADD CONSTRAINT skills_status_check CHECK (status IN ('draft', 'published'));
