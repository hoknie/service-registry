CREATE TABLE skill_blobs
(
    sha256             bytea PRIMARY KEY,
    size               bigint      NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    last_referenced_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT skill_blobs_sha256_check CHECK (octet_length(sha256) = 32),
    CONSTRAINT skill_blobs_size_check CHECK (size >= 0)
);
