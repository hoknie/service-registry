DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM personal_access_tokens WHERE node_ids IS NOT NULL AND revoked_at IS NULL) THEN
        RAISE EXCEPTION 'personal access tokens limited to catalog nodes are still active; revoke them before rolling back';
    END IF;
END
$$;

DROP TRIGGER nodes_forget_in_token_scopes ON nodes;
DROP FUNCTION nodes_forget_in_token_scopes();
DROP FUNCTION live_node_ids(uuid[]);

DROP FUNCTION agent_report_gap(integer, integer, integer);

ALTER TABLE knowledge_scans
    DROP COLUMN warning_counts,
    DROP COLUMN warning_details;

DELETE FROM knowledge_patterns
WHERE apis IS NOT NULL
  AND NOT include_set
  AND exclude IS NULL
  AND branches IS NULL;

ALTER TABLE knowledge_patterns
    DROP COLUMN apis;

DROP TABLE notification_episodes;
DROP TABLE group_contact_editors;
DROP TABLE node_owner_clears;

DROP TRIGGER service_dependencies_refresh_paths ON nodes;
DROP TABLE service_dependencies;
DROP FUNCTION service_dependencies_merge_unresolved();
DROP FUNCTION service_dependencies_refresh_paths();
DROP FUNCTION service_dependencies_target_path(uuid);

DROP TABLE link_target_projects;
DROP TABLE notification_deliveries;
DROP TABLE notification_events;
DROP TABLE notification_subscriptions;

ALTER TABLE cluster_workloads
    DROP COLUMN version_since;

ALTER TABLE groups
    DROP COLUMN on_call_url;

DROP TABLE group_contacts;
DROP TABLE node_owners;
DROP TABLE oauth_access_tokens;
DROP TABLE oauth_grants;
DROP TABLE oauth_authorizations;
DROP TABLE oauth_clients;

DROP FUNCTION viewer_bindings(uuid, uuid[], boolean);

ALTER TABLE personal_access_tokens
    DROP CONSTRAINT personal_access_tokens_scopes_check;

UPDATE personal_access_tokens
SET scopes = ARRAY(
    SELECT s
    FROM unnest(ARRAY['read', 'write', 'admin', 'mcp']::text[]) WITH ORDINALITY AS o(s, n)
    WHERE s = ANY (scopes)
        OR (s = 'admin' AND scopes && ARRAY['users:read', 'users:write', 'groups:read', 'groups:write', 'clusters:read',
            'clusters:write', 'settings:read', 'settings:write', 'catalog.root']::text[])
    ORDER BY n)
WHERE scopes && ARRAY['users:read', 'users:write', 'groups:read', 'groups:write', 'clusters:read', 'clusters:write',
    'settings:read', 'settings:write', 'catalog.root']::text[];

ALTER TABLE personal_access_tokens
    DROP COLUMN catalog_admin,
    DROP COLUMN node_ids,
    ADD CONSTRAINT personal_access_tokens_scopes_check
        CHECK (cardinality(scopes) > 0 AND scopes <@ ARRAY['read', 'write', 'admin', 'mcp']::text[]);

DROP TABLE cluster_agent_tokens;

DELETE FROM clusters
WHERE mode = 'agent';

ALTER TABLE clusters
    DROP CONSTRAINT clusters_credentials_check,
    DROP COLUMN mode,
    DROP COLUMN agent_version,
    DROP COLUMN agent_reported_at,
    DROP COLUMN agent_report,
    DROP COLUMN agent_workloads,
    ADD COLUMN credentials_enc text,
    ADD COLUMN credentials_ref text,
    ADD COLUMN credentials_fingerprint text,
    ADD CONSTRAINT clusters_credentials_ref_check CHECK (char_length(credentials_ref) <= 4101),
    ADD CONSTRAINT clusters_credentials_fingerprint_check CHECK (char_length(credentials_fingerprint) <= 64),
    ADD CONSTRAINT clusters_credentials_check CHECK (
        (in_cluster AND api_url IS NULL AND credentials_enc IS NULL AND credentials_ref IS NULL AND credentials_secret_id IS NULL)
            OR (NOT in_cluster AND api_url IS NOT NULL AND num_nonnulls(credentials_enc, credentials_ref, credentials_secret_id) = 1)
        );

ALTER TABLE knowledge_sources
    DROP CONSTRAINT knowledge_sources_fields_check,
    ADD COLUMN credentials_enc text,
    ADD COLUMN credentials_ref text,
    ADD COLUMN credentials_fingerprint text,
    ADD CONSTRAINT knowledge_sources_credentials_ref_check CHECK (char_length(credentials_ref) <= 4101),
    ADD CONSTRAINT knowledge_sources_credentials_fingerprint_check CHECK (char_length(credentials_fingerprint) <= 64),
    ADD CONSTRAINT knowledge_sources_fields_check CHECK (
        (kind = 'remote' AND forge IS NOT NULL AND url IS NOT NULL AND api_url IS NOT NULL AND path IS NULL)
            OR (kind <> 'remote' AND path IS NOT NULL AND forge IS NULL AND url IS NULL AND api_url IS NULL
            AND credentials_enc IS NULL AND credentials_ref IS NULL)),
    ADD CONSTRAINT knowledge_sources_credentials_check CHECK (num_nonnulls(credentials_enc, credentials_ref, credentials_secret_id) <= 1);

ALTER TABLE forge_connections
    DROP CONSTRAINT forge_connections_credentials_check,
    ADD COLUMN credentials_enc text,
    ADD COLUMN credentials_ref text,
    ADD COLUMN credentials_fingerprint text,
    ADD CONSTRAINT forge_connections_credentials_ref_check CHECK (char_length(credentials_ref) <= 4101),
    ADD CONSTRAINT forge_connections_credentials_fingerprint_check CHECK (char_length(credentials_fingerprint) <= 64),
    ADD CONSTRAINT forge_connections_credentials_check CHECK (num_nonnulls(credentials_enc, credentials_ref, credentials_secret_id) = 1);

DROP INDEX IF EXISTS nodes_slug_trgm_idx;
DROP INDEX IF EXISTS nodes_name_trgm_idx;
