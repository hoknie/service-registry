ALTER TABLE skill_folders
    ADD COLUMN disabled    boolean NOT NULL DEFAULT false,
    ADD COLUMN source_id   uuid,
    ADD COLUMN source_path text,
    ADD CONSTRAINT skill_folders_source_id_fkey FOREIGN KEY (source_id) REFERENCES skill_sources (id) ON DELETE CASCADE,
    ADD CONSTRAINT skill_folders_source_path_check CHECK (char_length(source_path) BETWEEN 1 AND 512),
    ADD CONSTRAINT skill_folders_source_check CHECK ((source_id IS NULL) = (source_path IS NULL));

CREATE UNIQUE INDEX skill_folders_source_path_idx ON skill_folders (source_id, source_path) WHERE source_id IS NOT NULL;

CREATE FUNCTION skill_folder_disabled(folder uuid) RETURNS uuid
    LANGUAGE sql
    STABLE
AS
$$
WITH RECURSIVE chain AS (SELECT id, parent_id, disabled, 0 AS depth
                         FROM skill_folders
                         WHERE id = folder
                         UNION ALL
                         SELECT f.id, f.parent_id, f.disabled, c.depth + 1
                         FROM skill_folders f
                                  JOIN chain c ON f.id = c.parent_id)
SELECT id
FROM chain
WHERE disabled
ORDER BY depth
LIMIT 1
$$;
