ALTER TABLE notification_events DROP CONSTRAINT notification_events_type_check;
UPDATE notification_events
SET type = CASE type WHEN 'forge.sync_failed' THEN 'repo_sync.failed' ELSE 'repo_sync.recovered' END,
    dedupe_key = regexp_replace(dedupe_key, '^forge:', 'repo_sync:'),
    payload = replace(replace(replace(replace(replace(payload::text, '"forge.interrupted"', '"repo_sync.interrupted"'), '"forge.sync_failed"', '"repo_sync.failed"'), '"forge.sync_recovered"', '"repo_sync.recovered"'), '"forge": ', '"hosting": '), '"forge.', '"hosting.')::jsonb
WHERE type IN ('forge.sync_failed', 'forge.sync_recovered');
ALTER TABLE notification_events ADD CONSTRAINT notification_events_type_check CHECK (type IN ('deployment.created', 'deployment.drift', 'deployment.drift_resolved', 'link.check_failed', 'link.check_recovered', 'repo_sync.failed', 'repo_sync.recovered', 'test'));
