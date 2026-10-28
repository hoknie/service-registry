ALTER TABLE notification_events DROP CONSTRAINT notification_events_type_check;
UPDATE notification_events
SET type = CASE type WHEN 'repo_sync.failed' THEN 'forge.sync_failed' ELSE 'forge.sync_recovered' END,
    dedupe_key = regexp_replace(dedupe_key, '^repo_sync:', 'forge:'),
    payload = replace(replace(replace(replace(replace(payload::text, '"repo_sync.interrupted"', '"forge.interrupted"'), '"repo_sync.failed"', '"forge.sync_failed"'), '"repo_sync.recovered"', '"forge.sync_recovered"'), '"hosting": ', '"forge": '), '"hosting.', '"forge.')::jsonb
WHERE type IN ('repo_sync.failed', 'repo_sync.recovered');
ALTER TABLE notification_events ADD CONSTRAINT notification_events_type_check CHECK (type IN ('deployment.created', 'deployment.drift', 'deployment.drift_resolved', 'link.check_failed', 'link.check_recovered', 'forge.sync_failed', 'forge.sync_recovered', 'test'));
