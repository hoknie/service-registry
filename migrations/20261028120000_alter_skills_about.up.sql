ALTER TABLE skills
    ADD COLUMN about text NOT NULL DEFAULT '',
    ADD CONSTRAINT skills_about_check CHECK (char_length(about) <= 4000);
