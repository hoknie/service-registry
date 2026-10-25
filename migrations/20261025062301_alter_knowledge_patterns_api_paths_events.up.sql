ALTER TABLE knowledge_patterns
    ADD COLUMN api_paths text[],
    ADD COLUMN refresh_events text[],
    ADD CONSTRAINT knowledge_patterns_api_paths_check CHECK (cardinality(api_paths) <= 50),
    ADD CONSTRAINT knowledge_patterns_refresh_events_check
        CHECK (refresh_events <@ ARRAY['service.deployed', 'cluster.deployed', 'forge.webhook', 'forge.manual']::text[]);
