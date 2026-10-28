ALTER TABLE notification_episodes DROP CONSTRAINT notification_episodes_type_check;
UPDATE notification_episodes SET type = 'repo_sync.failed' WHERE type = 'forge.sync_failed';
ALTER TABLE notification_episodes ADD CONSTRAINT notification_episodes_type_check CHECK (type IN ('deployment.drift', 'link.check_failed', 'repo_sync.failed'));
