DROP INDEX oauth_grants_mcp_server_ids_idx;

ALTER TABLE oauth_grants
    DROP COLUMN mcp_server_ids;
