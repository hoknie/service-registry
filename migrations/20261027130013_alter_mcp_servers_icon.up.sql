ALTER TABLE mcp_servers
    ADD COLUMN icon text,
    ADD CONSTRAINT mcp_servers_icon_check CHECK (icon ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(icon) <= 64);
