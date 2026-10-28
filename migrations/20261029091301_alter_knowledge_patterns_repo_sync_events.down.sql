ALTER TABLE knowledge_patterns DROP CONSTRAINT knowledge_patterns_refresh_events_check;
UPDATE knowledge_patterns SET refresh_events = array_replace(array_replace(refresh_events, 'repo_sync.webhook', 'forge.webhook'), 'repo_sync.manual', 'forge.manual')
WHERE refresh_events && ARRAY['repo_sync.webhook', 'repo_sync.manual']::text[];
ALTER TABLE knowledge_patterns ADD CONSTRAINT knowledge_patterns_refresh_events_check CHECK (refresh_events <@ ARRAY['service.deployed', 'cluster.deployed', 'forge.webhook', 'forge.manual']::text[]);
