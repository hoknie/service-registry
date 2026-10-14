ALTER TABLE users
    ADD CONSTRAINT users_email_check CHECK (char_length(email) BETWEEN 1 AND 254),
    ADD CONSTRAINT users_display_name_check CHECK (char_length(display_name) BETWEEN 1 AND 100),
    ADD CONSTRAINT users_password_hash_check CHECK (char_length(password_hash) <= 255);

ALTER TABLE groups
    ADD CONSTRAINT groups_name_check CHECK (char_length(name) BETWEEN 1 AND 100);

ALTER TABLE personal_access_tokens
    ADD CONSTRAINT personal_access_tokens_prefix_check CHECK (char_length(prefix) = 12);

DELETE FROM user_identities WHERE char_length(subject) > 255;
UPDATE user_identities SET email = NULL WHERE char_length(email) > 254;

ALTER TABLE user_identities
    ADD CONSTRAINT user_identities_provider_check CHECK (char_length(provider) BETWEEN 1 AND 32),
    ADD CONSTRAINT user_identities_subject_check CHECK (char_length(subject) BETWEEN 1 AND 255),
    ADD CONSTRAINT user_identities_email_check CHECK (char_length(email) <= 254);

ALTER TABLE oauth_login_states
    ADD CONSTRAINT oauth_login_states_provider_check CHECK (char_length(provider) BETWEEN 1 AND 32),
    ADD CONSTRAINT oauth_login_states_state_check CHECK (char_length(state) BETWEEN 1 AND 64),
    ADD CONSTRAINT oauth_login_states_nonce_check CHECK (char_length(nonce) BETWEEN 1 AND 64),
    ADD CONSTRAINT oauth_login_states_code_verifier_check CHECK (char_length(code_verifier) BETWEEN 1 AND 64),
    ADD CONSTRAINT oauth_login_states_next_check CHECK (char_length(next) <= 2048);

ALTER TABLE secrets
    ADD CONSTRAINT secrets_value_ref_check CHECK (char_length(value_ref) <= 4101),
    ADD CONSTRAINT secrets_fingerprint_check CHECK (char_length(fingerprint) <= 64);

UPDATE nodes SET default_branch = NULL WHERE char_length(default_branch) > 255;
UPDATE nodes SET repo_url = NULL WHERE char_length(repo_url) > 2048;

ALTER TABLE nodes
    ADD CONSTRAINT nodes_slug_check CHECK (char_length(slug) BETWEEN 1 AND 63),
    ADD CONSTRAINT nodes_name_check CHECK (char_length(name) BETWEEN 1 AND 100),
    ADD CONSTRAINT nodes_description_check CHECK (char_length(description) <= 2000),
    ADD CONSTRAINT nodes_repo_url_check CHECK (char_length(repo_url) <= 2048),
    ADD CONSTRAINT nodes_default_branch_check CHECK (char_length(default_branch) BETWEEN 1 AND 255);

ALTER TABLE node_vars
    ADD CONSTRAINT node_vars_key_check CHECK (char_length(key) BETWEEN 1 AND 63),
    ADD CONSTRAINT node_vars_value_check CHECK (char_length(value) <= 1024);

DELETE FROM branches WHERE char_length(name) > 255;

ALTER TABLE branches
    ADD CONSTRAINT branches_name_check CHECK (char_length(name) BETWEEN 1 AND 255);

ALTER TABLE project_events
    ADD CONSTRAINT project_events_idempotency_key_check CHECK (char_length(idempotency_key) BETWEEN 1 AND 200);

ALTER TABLE project_keys
    ADD CONSTRAINT project_keys_prefix_check CHECK (char_length(prefix) = 12);

DELETE FROM service_deployments WHERE source = 'cluster' AND char_length(version) > 255;

ALTER TABLE service_deployments
    ADD CONSTRAINT service_deployments_service_check CHECK (char_length(service) BETWEEN 1 AND 63),
    ADD CONSTRAINT service_deployments_environment_check CHECK (char_length(environment) BETWEEN 1 AND 63),
    ADD CONSTRAINT service_deployments_version_check CHECK (char_length(version) BETWEEN 1 AND 255),
    ADD CONSTRAINT service_deployments_commit_sha_check CHECK (commit_sha ~ '^[0-9a-f]{7,64}$'),
    ADD CONSTRAINT service_deployments_branch_check CHECK (char_length(branch) BETWEEN 1 AND 255),
    ADD CONSTRAINT service_deployments_cluster_check CHECK (char_length(cluster) <= 100),
    ADD CONSTRAINT service_deployments_namespace_check CHECK (char_length(namespace) <= 63),
    ADD CONSTRAINT service_deployments_url_check CHECK (char_length(url) <= 2048),
    ADD CONSTRAINT service_deployments_deployed_by_check CHECK (char_length(deployed_by) <= 200);

ALTER TABLE service_environments
    ADD CONSTRAINT service_environments_service_check CHECK (char_length(service) BETWEEN 1 AND 63),
    ADD CONSTRAINT service_environments_environment_check CHECK (char_length(environment) BETWEEN 1 AND 63);

ALTER TABLE forge_connections
    ADD CONSTRAINT forge_connections_api_url_check CHECK (char_length(api_url) BETWEEN 1 AND 2048),
    ADD CONSTRAINT forge_connections_owner_path_check CHECK (char_length(owner_path) BETWEEN 1 AND 255),
    ADD CONSTRAINT forge_connections_credentials_ref_check CHECK (char_length(credentials_ref) <= 4101),
    ADD CONSTRAINT forge_connections_credentials_fingerprint_check CHECK (char_length(credentials_fingerprint) <= 64),
    ADD CONSTRAINT forge_connections_webhook_id_check CHECK (char_length(webhook_id) <= 64);

DELETE FROM forge_groups WHERE char_length(full_path) > 1024;

ALTER TABLE forge_groups
    ADD CONSTRAINT forge_groups_full_path_check CHECK (char_length(full_path) BETWEEN 1 AND 1024);

DELETE FROM forge_repositories WHERE char_length(full_path) > 1024 OR char_length(web_url) > 2048;
UPDATE forge_repositories SET default_branch = NULL WHERE char_length(default_branch) > 255;
UPDATE forge_repositories SET license = NULL WHERE char_length(license) > 255;

ALTER TABLE forge_repositories
    ADD CONSTRAINT forge_repositories_external_id_check CHECK (char_length(external_id) BETWEEN 1 AND 64),
    ADD CONSTRAINT forge_repositories_full_path_check CHECK (char_length(full_path) BETWEEN 1 AND 1024),
    ADD CONSTRAINT forge_repositories_web_url_check CHECK (char_length(web_url) BETWEEN 1 AND 2048),
    ADD CONSTRAINT forge_repositories_default_branch_check CHECK (char_length(default_branch) BETWEEN 1 AND 255),
    ADD CONSTRAINT forge_repositories_license_check CHECK (char_length(license) <= 255);

ALTER TABLE forge_sync_runs
    ADD CONSTRAINT forge_sync_runs_error_code_check CHECK (char_length(error_code) <= 64);

DELETE FROM forge_exclusions WHERE char_length(full_path) > 1024;

ALTER TABLE forge_exclusions
    ADD CONSTRAINT forge_exclusions_external_id_check CHECK (char_length(external_id) BETWEEN 1 AND 64),
    ADD CONSTRAINT forge_exclusions_full_path_check CHECK (char_length(full_path) BETWEEN 1 AND 1024);

ALTER TABLE forge_adoptions
    ADD CONSTRAINT forge_adoptions_external_id_check CHECK (char_length(external_id) BETWEEN 1 AND 64);

ALTER TABLE link_kinds
    ADD CONSTRAINT link_kinds_key_check CHECK (char_length(key) BETWEEN 1 AND 63);

ALTER TABLE link_targets
    ADD CONSTRAINT link_targets_url_check CHECK (char_length(url) BETWEEN 1 AND 2048);

ALTER TABLE link_templates
    DROP CONSTRAINT link_templates_template_check,
    ADD CONSTRAINT link_templates_link_key_check CHECK (char_length(link_key) BETWEEN 1 AND 63),
    ADD CONSTRAINT link_templates_template_check CHECK (
        (template IS NULL) = disabled AND (template IS NULL OR char_length(template) BETWEEN 1 AND 2048));

ALTER TABLE environments
    ADD CONSTRAINT environments_key_check CHECK (char_length(key) BETWEEN 1 AND 63);

ALTER TABLE clusters
    ADD CONSTRAINT clusters_name_check CHECK (char_length(name) BETWEEN 1 AND 100),
    ADD CONSTRAINT clusters_environment_check CHECK (char_length(environment) BETWEEN 1 AND 63),
    ADD CONSTRAINT clusters_api_url_check CHECK (char_length(api_url) <= 2048),
    ADD CONSTRAINT clusters_credentials_ref_check CHECK (char_length(credentials_ref) <= 4101),
    ADD CONSTRAINT clusters_credentials_fingerprint_check CHECK (char_length(credentials_fingerprint) <= 64);

UPDATE cluster_workloads SET annotation = left(annotation, 1024) WHERE char_length(annotation) > 1024;
UPDATE cluster_workloads SET version = NULL WHERE char_length(version) > 255;
UPDATE cluster_workloads SET pending_version = NULL, pending_since = NULL WHERE char_length(pending_version) > 255;

ALTER TABLE cluster_workloads
    ADD CONSTRAINT cluster_workloads_uid_check CHECK (char_length(uid) BETWEEN 1 AND 64),
    ADD CONSTRAINT cluster_workloads_namespace_check CHECK (char_length(namespace) BETWEEN 1 AND 63),
    ADD CONSTRAINT cluster_workloads_name_check CHECK (char_length(name) BETWEEN 1 AND 253),
    ADD CONSTRAINT cluster_workloads_annotation_check CHECK (char_length(annotation) <= 1024),
    ADD CONSTRAINT cluster_workloads_service_check CHECK (char_length(service) <= 63),
    ADD CONSTRAINT cluster_workloads_environment_check CHECK (char_length(environment) <= 63),
    ADD CONSTRAINT cluster_workloads_branch_check CHECK (char_length(branch) <= 255),
    ADD CONSTRAINT cluster_workloads_version_check CHECK (char_length(version) <= 255),
    ADD CONSTRAINT cluster_workloads_pending_version_check CHECK (char_length(pending_version) <= 255);

DELETE FROM knowledge_snapshots WHERE char_length(branch) > 255;

ALTER TABLE knowledge_snapshots
    ADD CONSTRAINT knowledge_snapshots_branch_check CHECK (char_length(branch) BETWEEN 1 AND 255),
    ADD CONSTRAINT knowledge_snapshots_commit_sha_check CHECK (char_length(commit_sha) <= 255),
    ADD CONSTRAINT knowledge_snapshots_error_code_check CHECK (char_length(error_code) <= 64);

ALTER TABLE knowledge_files
    ADD CONSTRAINT knowledge_files_git_blob_sha_check CHECK (git_blob_sha ~ '^([0-9a-f]{40}|[0-9a-f]{64}|size:[0-9]{1,20})$');

ALTER TABLE knowledge_embeddings
    ADD CONSTRAINT knowledge_embeddings_model_check CHECK (char_length(model) BETWEEN 1 AND 255);

ALTER TABLE knowledge_index_state
    ADD CONSTRAINT knowledge_index_state_model_check CHECK (char_length(model) <= 255),
    ADD CONSTRAINT knowledge_index_state_fingerprint_check CHECK (char_length(fingerprint) <= 64),
    ADD CONSTRAINT knowledge_index_state_failure_check CHECK (char_length(failure) <= 64);

ALTER TABLE knowledge_scans
    ADD CONSTRAINT knowledge_scans_error_code_check CHECK (char_length(error_code) <= 64);

UPDATE knowledge_sources SET default_branch = NULL WHERE char_length(default_branch) > 255;

ALTER TABLE knowledge_sources
    ADD CONSTRAINT knowledge_sources_url_check CHECK (char_length(url) <= 2048),
    ADD CONSTRAINT knowledge_sources_api_url_check CHECK (char_length(api_url) <= 2048),
    ADD CONSTRAINT knowledge_sources_path_check CHECK (char_length(path) <= 4096),
    ADD CONSTRAINT knowledge_sources_credentials_ref_check CHECK (char_length(credentials_ref) <= 4101),
    ADD CONSTRAINT knowledge_sources_credentials_fingerprint_check CHECK (char_length(credentials_fingerprint) <= 64),
    ADD CONSTRAINT knowledge_sources_default_branch_check CHECK (char_length(default_branch) <= 255);

DELETE FROM knowledge_source_repos WHERE char_length(full_path) > 1024 OR char_length(web_url) > 2048;
UPDATE knowledge_source_repos SET default_branch = NULL WHERE char_length(default_branch) > 255;
UPDATE knowledge_source_repos SET license = NULL WHERE char_length(license) > 255;

ALTER TABLE knowledge_source_repos
    ADD CONSTRAINT knowledge_source_repos_full_path_check CHECK (char_length(full_path) BETWEEN 1 AND 1024),
    ADD CONSTRAINT knowledge_source_repos_web_url_check CHECK (char_length(web_url) BETWEEN 1 AND 2048),
    ADD CONSTRAINT knowledge_source_repos_default_branch_check CHECK (char_length(default_branch) BETWEEN 1 AND 255),
    ADD CONSTRAINT knowledge_source_repos_license_check CHECK (char_length(license) <= 255);
