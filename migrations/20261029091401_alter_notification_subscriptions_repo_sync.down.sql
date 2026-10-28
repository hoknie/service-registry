ALTER TABLE notification_subscriptions DROP CONSTRAINT notification_subscriptions_event_types_check;
UPDATE notification_subscriptions SET event_types = array_replace(array_replace(event_types, 'repo_sync.failed', 'forge.sync_failed'), 'repo_sync.recovered', 'forge.sync_recovered')
WHERE event_types && ARRAY['repo_sync.failed', 'repo_sync.recovered']::text[];
ALTER TABLE notification_subscriptions ADD CONSTRAINT notification_subscriptions_event_types_check CHECK (cardinality(event_types) BETWEEN 1 AND 7 AND event_types <@ ARRAY['deployment.created', 'deployment.drift', 'deployment.drift_resolved', 'link.check_failed', 'link.check_recovered', 'forge.sync_failed', 'forge.sync_recovered']::text[]);
