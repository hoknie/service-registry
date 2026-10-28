UPDATE personal_access_tokens
SET scopes = array_remove(scopes, 'skills:write')
WHERE 'skills:write' = ANY (scopes);

DELETE FROM personal_access_tokens
WHERE cardinality(scopes) = 0;

ALTER TABLE personal_access_tokens
    DROP CONSTRAINT personal_access_tokens_scopes_check,
    ADD CONSTRAINT personal_access_tokens_scopes_check
        CHECK (cardinality(scopes) > 0 AND scopes <@
            ARRAY['read', 'write', 'mcp', 'users:read', 'users:write', 'groups:read', 'groups:write',
                'clusters:read', 'clusters:write', 'settings:read', 'settings:write', 'catalog.root', 'publication:write']::text[]);
