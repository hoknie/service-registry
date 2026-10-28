CREATE TABLE mcp_organizations
(
    organization_id uuid PRIMARY KEY,
    description     text        NOT NULL DEFAULT '',
    set_mode        text        NOT NULL DEFAULT 'all',
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mcp_organizations_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT mcp_organizations_description_check CHECK (char_length(description) <= 2000),
    CONSTRAINT mcp_organizations_set_mode_check CHECK (set_mode IN ('all', 'selected'))
);
