ALTER TABLE branches DROP CONSTRAINT branches_sources_check;
ALTER TABLE branches
    ADD CONSTRAINT branches_sources_check
        CHECK (cardinality(sources) > 0 AND sources <@ ARRAY['forge', 'ingest', 'cluster', 'manual']::text[]);
