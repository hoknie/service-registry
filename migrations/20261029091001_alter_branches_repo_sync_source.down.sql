ALTER TABLE branches DROP CONSTRAINT branches_sources_check;
UPDATE branches SET sources = array_replace(sources, 'repo_sync', 'forge') WHERE 'repo_sync' = ANY(sources);
ALTER TABLE branches ADD CONSTRAINT branches_sources_check CHECK (cardinality(sources) > 0 AND sources <@ ARRAY['forge', 'repository', 'ingest', 'cluster', 'manual']::text[]);
