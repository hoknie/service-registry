ALTER TABLE notification_episodes DROP CONSTRAINT notification_episodes_type_check;
UPDATE notification_episodes SET type = 'forge.sync_failed' WHERE type = 'repo_sync.failed';
ALTER TABLE notification_episodes ADD CONSTRAINT notification_episodes_type_check CHECK (type IN ('deployment.drift', 'link.check_failed', 'forge.sync_failed'));
