CREATE TABLE knowledge_files
(
    id           uuid PRIMARY KEY,
    snapshot_id  uuid   NOT NULL,
    path         text   NOT NULL,
    git_blob_sha text   NOT NULL,
    sha256       bytea,
    bytes        bigint NOT NULL,
    skip_reason  text,
    kind         text   NOT NULL,
    meta         jsonb  NOT NULL DEFAULT '{}',
    CONSTRAINT knowledge_files_snapshot_id_fkey FOREIGN KEY (snapshot_id) REFERENCES knowledge_snapshots (id) ON DELETE CASCADE,
    CONSTRAINT knowledge_files_sha256_fkey FOREIGN KEY (sha256) REFERENCES knowledge_blobs (sha256) ON DELETE RESTRICT,
    CONSTRAINT knowledge_files_path_key UNIQUE (snapshot_id, path),
    CONSTRAINT knowledge_files_skip_reason_check CHECK (skip_reason IN ('too_large', 'binary', 'limit')),
    CONSTRAINT knowledge_files_content_check CHECK ((sha256 IS NULL) <> (skip_reason IS NULL)),
    CONSTRAINT knowledge_files_kind_check CHECK (kind IN ('doc', 'spec', 'change', 'adr'))
);

CREATE INDEX knowledge_files_sha256_idx ON knowledge_files (sha256);
CREATE INDEX knowledge_files_git_blob_sha_idx ON knowledge_files (git_blob_sha);
