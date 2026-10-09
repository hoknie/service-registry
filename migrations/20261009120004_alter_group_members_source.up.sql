ALTER TABLE group_members
    ADD COLUMN source text NOT NULL DEFAULT 'manual';
ALTER TABLE group_members
    ADD CONSTRAINT group_members_source_check
        CHECK (source = 'manual' OR source ~ '^oauth:[a-z0-9-]{1,32}$');
