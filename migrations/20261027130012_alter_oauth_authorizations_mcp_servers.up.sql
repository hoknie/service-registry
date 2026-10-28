ALTER TABLE oauth_authorizations
    ADD COLUMN mcp_server_ids uuid[],
    ADD CONSTRAINT oauth_authorizations_mcp_server_ids_check CHECK (cardinality(mcp_server_ids) <= 50);
