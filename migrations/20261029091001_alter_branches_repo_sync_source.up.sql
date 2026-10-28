ALTER TABLE branches DROP CONSTRAINT branches_sources_check;
UPDATE branches SET sources = array_replace(sources, 'forge', 'repo_sync') WHERE 'forge' = ANY(sources);
ALTER TABLE branches ADD CONSTRAINT branches_sources_check CHECK (cardinality(sources) > 0 AND sources <@ ARRAY['repo_sync', 'repository', 'ingest', 'cluster', 'manual']::text[]);
