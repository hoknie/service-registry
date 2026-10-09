CREATE TABLE knowledge_blobs
(
    sha256     bytea PRIMARY KEY,
    content    text        NOT NULL,
    bytes      bigint      NOT NULL,
    tsv        tsvector GENERATED ALWAYS AS (to_tsvector('simple', left(content, 262144))) STORED,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT knowledge_blobs_sha256_len_check CHECK (octet_length(sha256) = 32)
);

CREATE INDEX knowledge_blobs_tsv_idx ON knowledge_blobs USING gin (tsv);
