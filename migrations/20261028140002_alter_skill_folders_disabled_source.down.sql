DROP FUNCTION skill_folder_disabled(uuid);

DROP INDEX skill_folders_source_path_idx;

ALTER TABLE skill_folders
    DROP COLUMN source_path,
    DROP COLUMN source_id,
    DROP COLUMN disabled;
