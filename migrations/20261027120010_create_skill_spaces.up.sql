CREATE TABLE skill_spaces
(
    organization_id uuid PRIMARY KEY,
    description     text        NOT NULL DEFAULT '',
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT skill_spaces_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT skill_spaces_description_check CHECK (char_length(description) <= 2000)
);
