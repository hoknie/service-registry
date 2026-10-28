CREATE TABLE skill_files
(
    version_id   uuid   NOT NULL,
    path         text   NOT NULL,
    size         bigint NOT NULL,
    sha256       bytea  NOT NULL,
    kind         text   NOT NULL,
    content_type text   NOT NULL,
    title        text,
    PRIMARY KEY (version_id, path),
    CONSTRAINT skill_files_version_id_fkey FOREIGN KEY (version_id) REFERENCES skill_versions (id) ON DELETE CASCADE,
    CONSTRAINT skill_files_path_check CHECK (octet_length(path) BETWEEN 1 AND 512),
    CONSTRAINT skill_files_size_check CHECK (size >= 0),
    CONSTRAINT skill_files_sha256_check CHECK (octet_length(sha256) = 32),
    CONSTRAINT skill_files_kind_check CHECK (kind IN ('markdown', 'text', 'image', 'binary')),
    CONSTRAINT skill_files_content_type_check CHECK (char_length(content_type) BETWEEN 1 AND 100),
    CONSTRAINT skill_files_title_check CHECK (char_length(title) <= 200)
);

CREATE INDEX skill_files_sha256_idx ON skill_files (sha256);
