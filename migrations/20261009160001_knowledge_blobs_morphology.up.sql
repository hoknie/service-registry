DROP INDEX knowledge_blobs_tsv_idx;
ALTER TABLE knowledge_blobs DROP COLUMN tsv;
ALTER TABLE knowledge_blobs
    ADD COLUMN tsv tsvector GENERATED ALWAYS AS
        (to_tsvector('russian', left(content, 262144)) || to_tsvector('simple', left(content, 262144))) STORED;
CREATE INDEX knowledge_blobs_tsv_idx ON knowledge_blobs USING gin (tsv);
