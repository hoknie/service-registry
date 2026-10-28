ALTER TABLE personal_access_tokens
    DROP CONSTRAINT personal_access_tokens_scopes_check,
    ADD CONSTRAINT personal_access_tokens_scopes_check
        CHECK (cardinality(scopes) > 0 AND scopes <@
            ARRAY['read', 'write', 'mcp', 'users:read', 'users:write', 'groups:read', 'groups:write',
                'clusters:read', 'clusters:write', 'settings:read', 'settings:write', 'catalog.root', 'publication:write',
                'skills:write']::text[]);
