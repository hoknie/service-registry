ALTER TABLE link_kinds
    DROP CONSTRAINT link_kinds_icon_check;

UPDATE link_kinds
SET icon = CASE icon
               WHEN 'logs' THEN 'scroll-text'
               WHEN 'dashboard' THEN 'chart-line'
               WHEN 'errors' THEN 'bug'
               WHEN 'alerts' THEN 'bell-ring'
               WHEN 'traces' THEN 'waypoints'
               WHEN 'runbook' THEN 'book-open-check'
               WHEN 'docs' THEN 'book-text'
               ELSE icon
    END;

ALTER TABLE link_kinds
    ADD CONSTRAINT link_kinds_icon_check CHECK (length(icon) <= 64 AND icon ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    ADD COLUMN icon_url text,
    ADD CONSTRAINT link_kinds_icon_url_check CHECK (icon_url IS NULL OR (icon_url LIKE 'https://%' AND length(icon_url) <= 2048));
