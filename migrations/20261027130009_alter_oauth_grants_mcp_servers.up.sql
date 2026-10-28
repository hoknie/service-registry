ALTER TABLE oauth_grants
    ADD COLUMN mcp_server_ids uuid[],
    ADD CONSTRAINT oauth_grants_mcp_server_ids_check CHECK (cardinality(mcp_server_ids) <= 50);

CREATE INDEX oauth_grants_mcp_server_ids_idx ON oauth_grants USING gin (mcp_server_ids) WHERE mcp_server_ids IS NOT NULL;
