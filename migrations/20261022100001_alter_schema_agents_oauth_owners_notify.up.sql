DO
$$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'pg_trgm')
        AND NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') THEN
        BEGIN
            CREATE EXTENSION pg_trgm SCHEMA public;
        EXCEPTION WHEN insufficient_privilege OR unique_violation OR duplicate_object THEN
            NULL;
        END;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') THEN
        EXECUTE format('CREATE INDEX IF NOT EXISTS nodes_name_trgm_idx ON nodes USING gin (lower(name) %I.gin_trgm_ops)',
            (SELECT n.nspname FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = 'pg_trgm'));
        EXECUTE format('CREATE INDEX IF NOT EXISTS nodes_slug_trgm_idx ON nodes USING gin (slug %I.gin_trgm_ops)',
            (SELECT n.nspname FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = 'pg_trgm'));
    END IF;
END
$$;

ALTER TABLE forge_connections
    DROP CONSTRAINT forge_connections_credentials_check,
    DROP CONSTRAINT forge_connections_credentials_ref_check,
    DROP CONSTRAINT forge_connections_credentials_fingerprint_check,
    DROP COLUMN credentials_enc,
    DROP COLUMN credentials_ref,
    DROP COLUMN credentials_fingerprint,
    ADD CONSTRAINT forge_connections_credentials_check CHECK (credentials_secret_id IS NOT NULL);

ALTER TABLE knowledge_sources
    DROP CONSTRAINT knowledge_sources_credentials_check,
    DROP CONSTRAINT knowledge_sources_fields_check,
    DROP CONSTRAINT knowledge_sources_credentials_ref_check,
    DROP CONSTRAINT knowledge_sources_credentials_fingerprint_check,
    DROP COLUMN credentials_enc,
    DROP COLUMN credentials_ref,
    DROP COLUMN credentials_fingerprint,
    ADD CONSTRAINT knowledge_sources_fields_check CHECK (
        (kind = 'remote' AND forge IS NOT NULL AND url IS NOT NULL AND api_url IS NOT NULL AND path IS NULL)
            OR (kind <> 'remote' AND path IS NOT NULL AND forge IS NULL AND url IS NULL AND api_url IS NULL));

ALTER TABLE clusters
    DROP CONSTRAINT clusters_credentials_check,
    DROP CONSTRAINT clusters_credentials_ref_check,
    DROP CONSTRAINT clusters_credentials_fingerprint_check,
    DROP COLUMN credentials_enc,
    DROP COLUMN credentials_ref,
    DROP COLUMN credentials_fingerprint,
    ADD COLUMN mode text NOT NULL DEFAULT 'poll',
    ADD COLUMN agent_version text,
    ADD COLUMN agent_reported_at timestamptz,
    ADD COLUMN agent_report jsonb,
    ADD COLUMN agent_workloads integer NOT NULL DEFAULT 0,
    ADD CONSTRAINT clusters_mode_check CHECK (mode IN ('poll', 'agent')),
    ADD CONSTRAINT clusters_agent_version_check CHECK (char_length(agent_version) BETWEEN 1 AND 64),
    ADD CONSTRAINT clusters_agent_report_check CHECK (jsonb_typeof(agent_report) = 'object'),
    ADD CONSTRAINT clusters_agent_workloads_check CHECK (agent_workloads BETWEEN 0 AND 100000),
    ADD CONSTRAINT clusters_credentials_check CHECK (
        (mode = 'agent' AND NOT in_cluster AND api_url IS NULL AND ca_pem IS NULL AND credentials_secret_id IS NULL)
            OR (mode = 'poll' AND in_cluster AND api_url IS NULL AND credentials_secret_id IS NULL)
            OR (mode = 'poll' AND NOT in_cluster AND api_url IS NOT NULL AND credentials_secret_id IS NOT NULL)
        );

CREATE TABLE cluster_agent_tokens
(
    id           uuid PRIMARY KEY,
    cluster_id   uuid        NOT NULL,
    prefix       text        NOT NULL,
    token_hash   bytea       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz,
    revoked_at   timestamptz,
    last_used_at timestamptz,
    CONSTRAINT cluster_agent_tokens_cluster_id_fkey FOREIGN KEY (cluster_id) REFERENCES clusters (id) ON DELETE CASCADE,
    CONSTRAINT cluster_agent_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT cluster_agent_tokens_token_hash_len_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT cluster_agent_tokens_prefix_check CHECK (char_length(prefix) = 12)
);

CREATE INDEX cluster_agent_tokens_cluster_id_idx ON cluster_agent_tokens (cluster_id, created_at);

ALTER TABLE personal_access_tokens
    DROP CONSTRAINT personal_access_tokens_scopes_check,
    ADD COLUMN node_ids uuid[],
    ADD COLUMN catalog_admin boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT personal_access_tokens_node_ids_check
        CHECK (node_ids IS NULL OR cardinality(node_ids) BETWEEN 1 AND 20);

UPDATE personal_access_tokens
SET scopes = ARRAY(
    SELECT s
    FROM unnest(ARRAY['read', 'write', 'mcp', 'users:read', 'users:write', 'groups:read', 'groups:write',
        'clusters:read', 'clusters:write', 'settings:read', 'settings:write', 'catalog.root']::text[])
        WITH ORDINALITY AS o(s, n)
    WHERE s = ANY (scopes) OR (n > 3 AND 'admin' = ANY (scopes))
    ORDER BY n)
WHERE 'admin' = ANY (scopes);

ALTER TABLE personal_access_tokens
    ADD CONSTRAINT personal_access_tokens_scopes_check
        CHECK (cardinality(scopes) > 0 AND scopes <@
            ARRAY['read', 'write', 'mcp', 'users:read', 'users:write', 'groups:read', 'groups:write',
                'clusters:read', 'clusters:write', 'settings:read', 'settings:write', 'catalog.root']::text[]);

CREATE INDEX personal_access_tokens_node_ids_idx ON personal_access_tokens USING gin (node_ids) WHERE node_ids IS NOT NULL;

CREATE FUNCTION viewer_bindings(p_user uuid, p_roots uuid[], p_admin boolean)
RETURNS TABLE (node_id uuid, role text)
LANGUAGE sql STABLE
AS $$
WITH RECURSIVE own AS (
    SELECT b.node_id, b.role
    FROM role_bindings b
    WHERE b.user_id = p_user
    UNION ALL
    SELECT b.node_id, b.role
    FROM role_bindings b
    JOIN group_members gm ON gm.group_id = b.group_id
    WHERE gm.user_id = p_user
),
above AS (
    SELECT n.id AS root, n.id, n.parent_id
    FROM nodes n
    WHERE p_roots IS NOT NULL
        AND n.id = ANY (p_roots)
    UNION ALL
    SELECT a.root, p.id, p.parent_id
    FROM above a
    JOIN nodes p ON p.id = a.parent_id
),
below AS (
    SELECT o.node_id AS bound, o.role, n.parent_id
    FROM own o
    JOIN nodes n ON n.id = o.node_id
    WHERE p_roots IS NOT NULL
    UNION ALL
    SELECT b.bound, b.role, p.parent_id
    FROM below b
    JOIN nodes p ON p.id = b.parent_id
)
SELECT o.node_id, o.role
FROM own o
WHERE p_roots IS NULL
UNION ALL
SELECT a.root, o.role
FROM above a
JOIN own o ON o.node_id = a.id
UNION ALL
SELECT b.bound, b.role
FROM below b
WHERE b.parent_id = ANY (p_roots)
UNION ALL
SELECT n.id, 'admin'
FROM nodes n
WHERE p_admin
    AND (n.id = ANY (p_roots) OR (p_roots IS NULL AND n.parent_id IS NULL))
$$;

CREATE TABLE oauth_clients
(
    id                  uuid PRIMARY KEY,
    name                text,
    redirect_uris       text[]      NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    last_used_at        timestamptz,
    metadata_url        text,
    metadata_expires_at timestamptz,
    CONSTRAINT oauth_clients_name_check CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT oauth_clients_redirect_uris_check CHECK (
        cardinality(redirect_uris) BETWEEN 1 AND 10
            AND array_position(redirect_uris, NULL) IS NULL
            AND char_length(array_to_string(redirect_uris, '')) <= 20000),
    CONSTRAINT oauth_clients_metadata_url_check CHECK (char_length(metadata_url) BETWEEN 1 AND 2000),
    CONSTRAINT oauth_clients_metadata_expires_at_check CHECK ((metadata_url IS NULL) = (metadata_expires_at IS NULL))
);

CREATE INDEX oauth_clients_created_at_idx ON oauth_clients (created_at);
CREATE UNIQUE INDEX oauth_clients_metadata_url_key ON oauth_clients (metadata_url);

CREATE TABLE oauth_authorizations
(
    id             uuid PRIMARY KEY,
    client_id      uuid        NOT NULL,
    redirect_uri   text        NOT NULL,
    state          text,
    code_challenge text        NOT NULL,
    resource       text,
    scopes         text[]      NOT NULL,
    user_id        uuid,
    node_ids       uuid[],
    code_hash      bytea,
    grant_id       uuid,
    decided_at     timestamptz,
    exchanged_at   timestamptz,
    expires_at     timestamptz NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT oauth_authorizations_client_id_fkey FOREIGN KEY (client_id) REFERENCES oauth_clients (id) ON DELETE CASCADE,
    CONSTRAINT oauth_authorizations_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT oauth_authorizations_code_hash_key UNIQUE (code_hash),
    CONSTRAINT oauth_authorizations_code_hash_len_check CHECK (octet_length(code_hash) = 32),
    CONSTRAINT oauth_authorizations_redirect_uri_check CHECK (char_length(redirect_uri) BETWEEN 1 AND 2000),
    CONSTRAINT oauth_authorizations_state_check CHECK (char_length(state) <= 500),
    CONSTRAINT oauth_authorizations_code_challenge_check CHECK (code_challenge ~ '^[A-Za-z0-9_-]{43}$'),
    CONSTRAINT oauth_authorizations_resource_check CHECK (char_length(resource) BETWEEN 1 AND 2000),
    CONSTRAINT oauth_authorizations_scopes_check CHECK (scopes = '{mcp}'),
    CONSTRAINT oauth_authorizations_node_ids_check CHECK (cardinality(node_ids) BETWEEN 1 AND 20)
);

CREATE INDEX oauth_authorizations_expires_at_idx ON oauth_authorizations (expires_at);
CREATE INDEX oauth_authorizations_node_ids_idx ON oauth_authorizations USING gin (node_ids) WHERE node_ids IS NOT NULL;

CREATE TABLE oauth_grants
(
    id                    uuid PRIMARY KEY,
    user_id               uuid        NOT NULL,
    client_id             uuid        NOT NULL,
    scopes                text[]      NOT NULL,
    node_ids              uuid[],
    refresh_hash          bytea       NOT NULL,
    previous_refresh_hash bytea,
    refresh_expires_at    timestamptz NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    last_used_at          timestamptz,
    revoked_at            timestamptz,
    catalog_admin         boolean     NOT NULL DEFAULT false,
    CONSTRAINT oauth_grants_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT oauth_grants_client_id_fkey FOREIGN KEY (client_id) REFERENCES oauth_clients (id) ON DELETE CASCADE,
    CONSTRAINT oauth_grants_refresh_hash_key UNIQUE (refresh_hash),
    CONSTRAINT oauth_grants_refresh_hash_len_check CHECK (octet_length(refresh_hash) = 32),
    CONSTRAINT oauth_grants_previous_refresh_hash_len_check CHECK (octet_length(previous_refresh_hash) = 32),
    CONSTRAINT oauth_grants_scopes_check CHECK (scopes = '{mcp}'),
    CONSTRAINT oauth_grants_node_ids_check CHECK (cardinality(node_ids) BETWEEN 1 AND 20)
);

CREATE INDEX oauth_grants_user_id_idx ON oauth_grants (user_id, created_at);
CREATE INDEX oauth_grants_client_id_idx ON oauth_grants (client_id);
CREATE INDEX oauth_grants_previous_refresh_hash_idx ON oauth_grants (previous_refresh_hash);
CREATE INDEX oauth_grants_node_ids_idx ON oauth_grants USING gin (node_ids) WHERE node_ids IS NOT NULL;

CREATE TABLE oauth_access_tokens
(
    id         uuid PRIMARY KEY,
    grant_id   uuid        NOT NULL,
    token_hash bytea       NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT oauth_access_tokens_grant_id_fkey FOREIGN KEY (grant_id) REFERENCES oauth_grants (id) ON DELETE CASCADE,
    CONSTRAINT oauth_access_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT oauth_access_tokens_token_hash_len_check CHECK (octet_length(token_hash) = 32)
);

CREATE INDEX oauth_access_tokens_grant_id_idx ON oauth_access_tokens (grant_id);
CREATE INDEX oauth_access_tokens_expires_at_idx ON oauth_access_tokens (expires_at);

CREATE TABLE node_owners
(
    id         uuid PRIMARY KEY,
    node_id    uuid        NOT NULL,
    group_id   uuid,
    user_id    uuid,
    position   smallint    NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT node_owners_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT node_owners_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
    CONSTRAINT node_owners_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT node_owners_subject_check CHECK (num_nonnulls(group_id, user_id) = 1),
    CONSTRAINT node_owners_position_check CHECK (position BETWEEN 0 AND 9),
    CONSTRAINT node_owners_node_group_key UNIQUE (node_id, group_id),
    CONSTRAINT node_owners_node_user_key UNIQUE (node_id, user_id),
    CONSTRAINT node_owners_node_position_key UNIQUE (node_id, position)
);

CREATE INDEX node_owners_group_id_idx ON node_owners (group_id);
CREATE INDEX node_owners_user_id_idx ON node_owners (user_id);

CREATE TABLE group_contacts
(
    id         uuid PRIMARY KEY,
    group_id   uuid        NOT NULL,
    kind       text        NOT NULL,
    value      text        NOT NULL,
    label      text,
    position   smallint    NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT group_contacts_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
    CONSTRAINT group_contacts_kind_check CHECK (kind IN ('email', 'chat', 'url', 'phone')),
    CONSTRAINT group_contacts_value_check CHECK (char_length(value) BETWEEN 1 AND 2048),
    CONSTRAINT group_contacts_label_check CHECK (char_length(label) BETWEEN 1 AND 100),
    CONSTRAINT group_contacts_position_check CHECK (position BETWEEN 0 AND 9),
    CONSTRAINT group_contacts_group_position_key UNIQUE (group_id, position)
);

ALTER TABLE groups
    ADD COLUMN on_call_url text,
    ADD CONSTRAINT groups_on_call_url_check CHECK (char_length(on_call_url) BETWEEN 1 AND 2048);

ALTER TABLE cluster_workloads
    ADD COLUMN version_since timestamptz NOT NULL DEFAULT now();

CREATE TABLE notification_subscriptions
(
    id                  uuid PRIMARY KEY,
    node_id             uuid        NOT NULL,
    key                 text        NOT NULL,
    channel             text        NOT NULL,
    secret_id           uuid,
    signing_secret_enc  text,
    signing_fingerprint text,
    event_types         text[]      NOT NULL,
    environments        text[],
    locale              text        NOT NULL DEFAULT 'en',
    disabled            boolean     NOT NULL DEFAULT false,
    last_test_at        timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notification_subscriptions_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT notification_subscriptions_secret_id_fkey FOREIGN KEY (secret_id) REFERENCES secrets (id),
    CONSTRAINT notification_subscriptions_node_key_key UNIQUE (node_id, key),
    CONSTRAINT notification_subscriptions_key_check CHECK (char_length(key) BETWEEN 1 AND 63),
    CONSTRAINT notification_subscriptions_channel_check CHECK (channel IN ('webhook', 'slack')),
    CONSTRAINT notification_subscriptions_signing_fingerprint_check CHECK (char_length(signing_fingerprint) <= 64),
    CONSTRAINT notification_subscriptions_locale_check CHECK (locale IN ('en', 'es', 'ru', 'zh')),
    CONSTRAINT notification_subscriptions_event_types_check CHECK (
        cardinality(event_types) BETWEEN 1 AND 7
        AND event_types <@ ARRAY ['deployment.created', 'deployment.drift', 'deployment.drift_resolved', 'link.check_failed',
            'link.check_recovered', 'forge.sync_failed', 'forge.sync_recovered']::text[]),
    CONSTRAINT notification_subscriptions_environments_check CHECK (cardinality(environments) <= 20),
    CONSTRAINT notification_subscriptions_secret_check CHECK (disabled OR secret_id IS NOT NULL),
    CONSTRAINT notification_subscriptions_signing_check CHECK (channel = 'webhook' OR signing_secret_enc IS NULL)
);

CREATE INDEX notification_subscriptions_secret_id_idx ON notification_subscriptions (secret_id);

CREATE TABLE notification_events
(
    id          uuid PRIMARY KEY,
    type        text        NOT NULL,
    node_id     uuid        NOT NULL,
    payload     jsonb       NOT NULL,
    dedupe_key  text        NOT NULL,
    occurred_at timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notification_events_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT notification_events_dedupe_key_key UNIQUE (dedupe_key),
    CONSTRAINT notification_events_type_check
        CHECK (type IN ('deployment.created', 'deployment.drift', 'deployment.drift_resolved', 'link.check_failed',
            'link.check_recovered', 'forge.sync_failed', 'forge.sync_recovered', 'test')),
    CONSTRAINT notification_events_dedupe_key_check CHECK (char_length(dedupe_key) BETWEEN 1 AND 200)
);

CREATE INDEX notification_events_node_id_idx ON notification_events (node_id);
CREATE INDEX notification_events_created_at_idx ON notification_events (created_at);

CREATE TABLE notification_deliveries
(
    id                 uuid PRIMARY KEY,
    event_id           uuid        NOT NULL,
    subscription_id    uuid        NOT NULL,
    status             text        NOT NULL DEFAULT 'pending',
    attempts           integer     NOT NULL DEFAULT 0,
    next_run_at        timestamptz NOT NULL DEFAULT now(),
    lease_until        timestamptz,
    last_http_status   integer,
    last_error_code    text,
    last_error_message text,
    delivered_at       timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notification_deliveries_event_id_fkey
        FOREIGN KEY (event_id) REFERENCES notification_events (id) ON DELETE CASCADE,
    CONSTRAINT notification_deliveries_subscription_id_fkey
        FOREIGN KEY (subscription_id) REFERENCES notification_subscriptions (id) ON DELETE CASCADE,
    CONSTRAINT notification_deliveries_event_subscription_key UNIQUE (event_id, subscription_id),
    CONSTRAINT notification_deliveries_status_check CHECK (status IN ('pending', 'delivered', 'failed')),
    CONSTRAINT notification_deliveries_attempts_check CHECK (attempts >= 0),
    CONSTRAINT notification_deliveries_last_error_code_check CHECK (char_length(last_error_code) <= 64)
);

CREATE INDEX notification_deliveries_status_next_run_at_idx ON notification_deliveries (status, next_run_at);
CREATE INDEX notification_deliveries_subscription_id_idx ON notification_deliveries (subscription_id, created_at);

CREATE TABLE link_target_projects
(
    id           uuid PRIMARY KEY,
    target_id    uuid        NOT NULL,
    project_id   uuid        NOT NULL,
    link_key     text        NOT NULL,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT link_target_projects_target_id_fkey FOREIGN KEY (target_id) REFERENCES link_targets (id) ON DELETE CASCADE,
    CONSTRAINT link_target_projects_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT link_target_projects_target_project_key_key UNIQUE (target_id, project_id, link_key),
    CONSTRAINT link_target_projects_link_key_check CHECK (char_length(link_key) BETWEEN 1 AND 63)
);

CREATE INDEX link_target_projects_project_id_idx ON link_target_projects (project_id);

CREATE TABLE service_dependencies
(
    id                uuid PRIMARY KEY,
    project_id        uuid        NOT NULL,
    source            text        NOT NULL,
    origin            text        NOT NULL,
    target_project_id uuid,
    target_external   text,
    target_ref        text        NOT NULL,
    note              text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    seen_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT service_dependencies_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT service_dependencies_target_project_id_fkey FOREIGN KEY (target_project_id) REFERENCES nodes (id) ON DELETE SET NULL,
    CONSTRAINT service_dependencies_source_check CHECK (source IN ('event', 'cluster', 'repository', 'manual')),
    CONSTRAINT service_dependencies_origin_check CHECK (
        char_length(origin) <= 600 AND (origin = '') = (source = 'manual')),
    CONSTRAINT service_dependencies_target_external_check CHECK (
        target_external ~ '^[a-z0-9]([a-z0-9._-]{0,61}[a-z0-9])?$'),
    CONSTRAINT service_dependencies_target_ref_check CHECK (char_length(target_ref) BETWEEN 1 AND 1024),
    CONSTRAINT service_dependencies_note_check CHECK (
        note IS NULL OR (source = 'manual' AND char_length(note) <= 500)),
    CONSTRAINT service_dependencies_target_check CHECK (
        NOT (target_project_id IS NOT NULL AND target_external IS NOT NULL)),
    CONSTRAINT service_dependencies_self_check CHECK (target_project_id IS DISTINCT FROM project_id)
);

CREATE UNIQUE INDEX service_dependencies_target_key ON service_dependencies (project_id, source, origin,
    (coalesce(target_project_id::text, 'external:' || target_external, 'ref:' || target_ref)));
CREATE INDEX service_dependencies_target_project_id_idx ON service_dependencies (target_project_id);

CREATE FUNCTION service_dependencies_target_path(p_id uuid)
RETURNS text
LANGUAGE sql STABLE
AS $$
WITH RECURSIVE up AS (
    SELECT n.parent_id, n.slug, 0 AS lvl
    FROM nodes n
    WHERE n.id = p_id
    UNION ALL
    SELECT p.parent_id, p.slug, up.lvl + 1
    FROM up
    JOIN nodes p ON p.id = up.parent_id
)
SELECT left(string_agg(slug, '/' ORDER BY lvl DESC), 1024)
FROM up
$$;

CREATE FUNCTION service_dependencies_refresh_paths()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    WITH RECURSIVE down AS (
        SELECT NEW.id AS id
        UNION ALL
        SELECT c.id
        FROM nodes c
        JOIN down d ON c.parent_id = d.id
    ),
    paths AS (
        SELECT d.id, service_dependencies_target_path(d.id) AS path
        FROM down d
        WHERE EXISTS (SELECT 1 FROM service_dependencies s WHERE s.target_project_id = d.id)
    )
    UPDATE service_dependencies s
    SET target_ref = p.path
    FROM paths p
    WHERE s.target_project_id = p.id
        AND p.path IS NOT NULL
        AND s.target_ref IS DISTINCT FROM p.path;
    RETURN NULL;
END
$$;

CREATE TRIGGER service_dependencies_refresh_paths
    AFTER UPDATE OF slug, parent_id ON nodes
    FOR EACH ROW
    WHEN (OLD.slug IS DISTINCT FROM NEW.slug OR OLD.parent_id IS DISTINCT FROM NEW.parent_id)
    EXECUTE FUNCTION service_dependencies_refresh_paths();

CREATE FUNCTION service_dependencies_merge_unresolved()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    v_note text;
BEGIN
    WITH gone AS (
        DELETE FROM service_dependencies d
        WHERE d.project_id = NEW.project_id
            AND d.source = NEW.source
            AND d.origin = NEW.origin
            AND d.target_project_id IS NULL
            AND d.target_external IS NULL
            AND d.target_ref = NEW.target_ref
            AND d.id <> NEW.id
        RETURNING d.note
    )
    SELECT max(note) INTO v_note FROM gone;
    NEW.note := coalesce(NEW.note, v_note);
    RETURN NEW;
END
$$;

CREATE TRIGGER service_dependencies_merge_unresolved
    BEFORE UPDATE OF target_project_id ON service_dependencies
    FOR EACH ROW
    WHEN (OLD.target_project_id IS NOT NULL AND NEW.target_project_id IS NULL AND NEW.target_external IS NULL)
    EXECUTE FUNCTION service_dependencies_merge_unresolved();

CREATE TABLE node_owner_clears
(
    node_id    uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT node_owner_clears_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE
);

CREATE TABLE group_contact_editors
(
    group_id   uuid        NOT NULL,
    user_id    uuid        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    source     text        NOT NULL DEFAULT 'manual',
    CONSTRAINT group_contact_editors_pkey PRIMARY KEY (group_id, user_id),
    CONSTRAINT group_contact_editors_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
    CONSTRAINT group_contact_editors_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT group_contact_editors_source_check CHECK (source = 'manual' OR source ~ '^oauth:[a-z0-9-]{1,32}$')
);

CREATE INDEX group_contact_editors_user_id_idx ON group_contact_editors (user_id);

CREATE TABLE notification_episodes
(
    key        text        PRIMARY KEY,
    id         uuid        NOT NULL,
    type       text        NOT NULL,
    node_id    uuid        NOT NULL,
    signature  text        NOT NULL,
    event_id   uuid        NOT NULL,
    opened_at  timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notification_episodes_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT notification_episodes_event_id_fkey FOREIGN KEY (event_id) REFERENCES notification_events (id) ON DELETE CASCADE
        DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT notification_episodes_key_check CHECK (char_length(key) BETWEEN 1 AND 200),
    CONSTRAINT notification_episodes_type_check CHECK (type IN ('deployment.drift', 'link.check_failed', 'forge.sync_failed')),
    CONSTRAINT notification_episodes_signature_check CHECK (char_length(signature) BETWEEN 1 AND 200)
);

CREATE INDEX notification_episodes_type_idx ON notification_episodes (type);
CREATE INDEX notification_episodes_event_id_idx ON notification_episodes (event_id);
CREATE INDEX notification_episodes_node_id_idx ON notification_episodes (node_id);

ALTER TABLE knowledge_patterns
    ADD COLUMN apis boolean;

ALTER TABLE knowledge_scans
    ADD COLUMN warning_details jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN warning_counts  jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD CONSTRAINT knowledge_scans_warning_details_check CHECK (jsonb_typeof(warning_details) = 'array' AND jsonb_array_length(warning_details) <= 20),
    ADD CONSTRAINT knowledge_scans_warning_counts_check CHECK (jsonb_typeof(warning_counts) = 'object');

CREATE FUNCTION agent_report_gap(interval_secs integer, workloads integer, workloads_per_sec integer) RETURNS integer
LANGUAGE sql IMMUTABLE PARALLEL SAFE
RETURN GREATEST(15, interval_secs / 2, ceil(workloads::numeric / workloads_per_sec)::integer);

CREATE FUNCTION live_node_ids(p_ids uuid[])
RETURNS uuid[]
LANGUAGE sql STABLE
AS $$
SELECT coalesce(array_agg(u.id ORDER BY u.ord), '{}')
FROM unnest(p_ids) WITH ORDINALITY AS u(id, ord)
WHERE EXISTS (SELECT 1 FROM nodes n WHERE n.id = u.id)
$$;

CREATE FUNCTION nodes_forget_in_token_scopes()
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
    SET node_ids = CASE WHEN cardinality(live_node_ids(node_ids)) > 0 THEN live_node_ids(node_ids) ELSE node_ids[1:1] END
    WHERE node_ids && v_gone;
    UPDATE oauth_grants
    SET node_ids = CASE WHEN cardinality(live_node_ids(node_ids)) > 0 THEN live_node_ids(node_ids) ELSE node_ids[1:1] END
    WHERE node_ids && v_gone;
    UPDATE oauth_authorizations
    SET node_ids = CASE WHEN cardinality(live_node_ids(node_ids)) > 0 THEN live_node_ids(node_ids) ELSE node_ids[1:1] END
    WHERE node_ids && v_gone;
    RETURN NULL;
END
$$;

CREATE TRIGGER nodes_forget_in_token_scopes
    AFTER DELETE ON nodes
    REFERENCING OLD TABLE AS gone
    FOR EACH STATEMENT
    EXECUTE FUNCTION nodes_forget_in_token_scopes();
