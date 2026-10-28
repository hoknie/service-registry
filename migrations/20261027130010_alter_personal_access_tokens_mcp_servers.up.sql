ALTER TABLE personal_access_tokens
    ADD COLUMN mcp_server_ids uuid[],
    ADD CONSTRAINT personal_access_tokens_mcp_server_ids_check CHECK (cardinality(mcp_server_ids) <= 50),
    DROP CONSTRAINT personal_access_tokens_scopes_check,
    ADD CONSTRAINT personal_access_tokens_scopes_check
        CHECK (cardinality(scopes) > 0 AND scopes <@
            ARRAY['read', 'write', 'mcp', 'users:read', 'users:write', 'groups:read', 'groups:write',
                'clusters:read', 'clusters:write', 'settings:read', 'settings:write', 'catalog.root', 'publication:write',
                'skills:write', 'mcp_servers:write']::text[]);

CREATE INDEX personal_access_tokens_mcp_server_ids_idx ON personal_access_tokens USING gin (mcp_server_ids)
    WHERE mcp_server_ids IS NOT NULL;

CREATE FUNCTION mcp_servers_forget_in_tokens()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    v_gone uuid[];
BEGIN
    SELECT array_agg(id) INTO v_gone FROM gone;
    IF v_gone IS NULL THEN
        RETURN NULL;
    END IF;
    UPDATE personal_access_tokens
    SET mcp_server_ids = ARRAY(SELECT s FROM unnest(mcp_server_ids) WITH ORDINALITY AS u(s, n) WHERE s <> ALL (v_gone) ORDER BY n)
    WHERE mcp_server_ids && v_gone;
    UPDATE oauth_grants
    SET mcp_server_ids = ARRAY(SELECT s FROM unnest(mcp_server_ids) WITH ORDINALITY AS u(s, n) WHERE s <> ALL (v_gone) ORDER BY n)
    WHERE mcp_server_ids && v_gone;
    RETURN NULL;
END
$$;

CREATE TRIGGER mcp_servers_forget_in_tokens
    AFTER DELETE ON mcp_servers
    REFERENCING OLD TABLE AS gone
    FOR EACH STATEMENT
    EXECUTE FUNCTION mcp_servers_forget_in_tokens();
