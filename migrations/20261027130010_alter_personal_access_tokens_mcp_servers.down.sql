DROP TRIGGER mcp_servers_forget_in_tokens ON mcp_servers;

DROP FUNCTION mcp_servers_forget_in_tokens();

UPDATE personal_access_tokens
SET scopes = array_remove(scopes, 'mcp_servers:write')
WHERE 'mcp_servers:write' = ANY (scopes);

DELETE FROM personal_access_tokens
WHERE cardinality(scopes) = 0;

DROP INDEX personal_access_tokens_mcp_server_ids_idx;

ALTER TABLE personal_access_tokens
    DROP COLUMN mcp_server_ids,
    DROP CONSTRAINT personal_access_tokens_scopes_check,
    ADD CONSTRAINT personal_access_tokens_scopes_check
        CHECK (cardinality(scopes) > 0 AND scopes <@
            ARRAY['read', 'write', 'mcp', 'users:read', 'users:write', 'groups:read', 'groups:write',
                'clusters:read', 'clusters:write', 'settings:read', 'settings:write', 'catalog.root', 'publication:write',
                'skills:write']::text[]);
