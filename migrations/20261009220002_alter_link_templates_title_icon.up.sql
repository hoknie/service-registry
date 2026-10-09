ALTER TABLE link_templates
    ADD COLUMN title text,
    ADD COLUMN icon_url text,
    ADD COLUMN icon_file text,
    ADD CONSTRAINT link_templates_title_check CHECK (title IS NULL OR char_length(title) BETWEEN 1 AND 100),
    ADD CONSTRAINT link_templates_icon_check CHECK (num_nulls(icon_url, icon_file) >= 1),
    ADD CONSTRAINT link_templates_icon_url_check CHECK (icon_url IS NULL OR (icon_url LIKE 'https://%' AND char_length(icon_url) <= 2048)),
    ADD CONSTRAINT link_templates_icon_file_check CHECK (icon_file IS NULL OR icon_file ~ '^[0-9a-f]{64}\.(png|webp|ico|svg)$');
