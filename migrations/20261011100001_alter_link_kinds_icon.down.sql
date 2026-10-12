ALTER TABLE link_kinds
    DROP CONSTRAINT link_kinds_icon_check,
    DROP CONSTRAINT link_kinds_icon_url_check,
    DROP COLUMN icon_url;

UPDATE link_kinds
SET icon = CASE icon
               WHEN 'scroll-text' THEN 'logs'
               WHEN 'chart-line' THEN 'dashboard'
               WHEN 'bug' THEN 'errors'
               WHEN 'bell-ring' THEN 'alerts'
               WHEN 'waypoints' THEN 'traces'
               WHEN 'book-open-check' THEN 'runbook'
               WHEN 'book-text' THEN 'docs'
               ELSE 'link'
    END;

ALTER TABLE link_kinds
    ADD CONSTRAINT link_kinds_icon_check
        CHECK (icon IN ('link', 'logs', 'dashboard', 'errors', 'alerts', 'traces', 'runbook', 'docs'));
