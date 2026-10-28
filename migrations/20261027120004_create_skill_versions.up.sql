CREATE TABLE skill_versions
(
    id         uuid PRIMARY KEY,
    skill_id   uuid        NOT NULL,
    version    text        NOT NULL,
    major      bigint      NOT NULL,
    minor      bigint      NOT NULL,
    patch      bigint      NOT NULL,
    prerelease text        NOT NULL DEFAULT '',
    origin     text        NOT NULL,
    commit_sha text,
    tag        text,
    digest     text        NOT NULL,
    yanked     boolean     NOT NULL DEFAULT false,
    files      integer     NOT NULL,
    size       bigint      NOT NULL,
    manifest   jsonb       NOT NULL,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT skill_versions_skill_id_fkey FOREIGN KEY (skill_id) REFERENCES skills (id) ON DELETE CASCADE,
    CONSTRAINT skill_versions_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT skill_versions_version_check CHECK (char_length(version) <= 128
        AND version ~ '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'),
    CONSTRAINT skill_versions_numbers_check CHECK (major >= 0 AND minor >= 0 AND patch >= 0),
    CONSTRAINT skill_versions_prerelease_check CHECK (char_length(prerelease) <= 100),
    CONSTRAINT skill_versions_origin_check CHECK (origin IN ('upload', 'git')),
    CONSTRAINT skill_versions_commit_sha_check CHECK (char_length(commit_sha) <= 200),
    CONSTRAINT skill_versions_tag_check CHECK (char_length(tag) <= 255),
    CONSTRAINT skill_versions_digest_check CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
    CONSTRAINT skill_versions_files_check CHECK (files >= 1),
    CONSTRAINT skill_versions_size_check CHECK (size >= 0),
    CONSTRAINT skill_versions_manifest_check CHECK (jsonb_typeof(manifest) = 'object')
);

CREATE UNIQUE INDEX skill_versions_skill_version_idx ON skill_versions (skill_id, version);
