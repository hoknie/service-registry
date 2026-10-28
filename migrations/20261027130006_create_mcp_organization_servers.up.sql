CREATE TABLE mcp_organization_servers
(
    organization_id uuid NOT NULL,
    server_id       uuid NOT NULL,
    CONSTRAINT mcp_organization_servers_pkey PRIMARY KEY (organization_id, server_id),
    CONSTRAINT mcp_organization_servers_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT mcp_organization_servers_server_id_fkey FOREIGN KEY (server_id) REFERENCES mcp_servers (id) ON DELETE CASCADE
);

CREATE INDEX mcp_organization_servers_server_idx ON mcp_organization_servers (server_id);
