ALTER TABLE mcp_server_checks
    ADD COLUMN icon bytea,
    ADD COLUMN icon_type text,
    ADD CONSTRAINT mcp_server_checks_icon_check CHECK (octet_length(icon) BETWEEN 1 AND 65536),
    ADD CONSTRAINT mcp_server_checks_icon_type_check CHECK (icon_type IN ('image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/svg+xml', 'image/x-icon')),
    ADD CONSTRAINT mcp_server_checks_icon_pair_check CHECK ((icon IS NULL) = (icon_type IS NULL));
