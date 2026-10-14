ALTER TABLE knowledge_source_repos
    DROP CONSTRAINT knowledge_source_repos_full_path_check,
    DROP CONSTRAINT knowledge_source_repos_web_url_check,
    DROP CONSTRAINT knowledge_source_repos_default_branch_check,
    DROP CONSTRAINT knowledge_source_repos_license_check;

ALTER TABLE knowledge_sources
    DROP CONSTRAINT knowledge_sources_url_check,
    DROP CONSTRAINT knowledge_sources_api_url_check,
    DROP CONSTRAINT knowledge_sources_path_check,
    DROP CONSTRAINT knowledge_sources_credentials_ref_check,
    DROP CONSTRAINT knowledge_sources_credentials_fingerprint_check,
    DROP CONSTRAINT knowledge_sources_default_branch_check;

ALTER TABLE knowledge_scans
    DROP CONSTRAINT knowledge_scans_error_code_check;

ALTER TABLE knowledge_index_state
    DROP CONSTRAINT knowledge_index_state_model_check,
    DROP CONSTRAINT knowledge_index_state_fingerprint_check,
    DROP CONSTRAINT knowledge_index_state_failure_check;

ALTER TABLE knowledge_embeddings
    DROP CONSTRAINT knowledge_embeddings_model_check;

ALTER TABLE knowledge_files
    DROP CONSTRAINT knowledge_files_git_blob_sha_check;

ALTER TABLE knowledge_snapshots
    DROP CONSTRAINT knowledge_snapshots_branch_check,
    DROP CONSTRAINT knowledge_snapshots_commit_sha_check,
    DROP CONSTRAINT knowledge_snapshots_error_code_check;

ALTER TABLE cluster_workloads
    DROP CONSTRAINT cluster_workloads_uid_check,
    DROP CONSTRAINT cluster_workloads_namespace_check,
    DROP CONSTRAINT cluster_workloads_name_check,
    DROP CONSTRAINT cluster_workloads_annotation_check,
    DROP CONSTRAINT cluster_workloads_service_check,
    DROP CONSTRAINT cluster_workloads_environment_check,
    DROP CONSTRAINT cluster_workloads_branch_check,
    DROP CONSTRAINT cluster_workloads_version_check,
    DROP CONSTRAINT cluster_workloads_pending_version_check;

ALTER TABLE clusters
    DROP CONSTRAINT clusters_name_check,
    DROP CONSTRAINT clusters_environment_check,
    DROP CONSTRAINT clusters_api_url_check,
    DROP CONSTRAINT clusters_credentials_ref_check,
    DROP CONSTRAINT clusters_credentials_fingerprint_check;

ALTER TABLE environments
    DROP CONSTRAINT environments_key_check;

ALTER TABLE link_templates
    DROP CONSTRAINT link_templates_link_key_check,
    DROP CONSTRAINT link_templates_template_check,
    ADD CONSTRAINT link_templates_template_check CHECK ((template IS NULL) = disabled);

ALTER TABLE link_targets
    DROP CONSTRAINT link_targets_url_check;

ALTER TABLE link_kinds
    DROP CONSTRAINT link_kinds_key_check;

ALTER TABLE forge_adoptions
    DROP CONSTRAINT forge_adoptions_external_id_check;

ALTER TABLE forge_exclusions
    DROP CONSTRAINT forge_exclusions_external_id_check,
    DROP CONSTRAINT forge_exclusions_full_path_check;

ALTER TABLE forge_sync_runs
    DROP CONSTRAINT forge_sync_runs_error_code_check;

ALTER TABLE forge_repositories
    DROP CONSTRAINT forge_repositories_external_id_check,
    DROP CONSTRAINT forge_repositories_full_path_check,
    DROP CONSTRAINT forge_repositories_web_url_check,
    DROP CONSTRAINT forge_repositories_default_branch_check,
    DROP CONSTRAINT forge_repositories_license_check;

ALTER TABLE forge_groups
    DROP CONSTRAINT forge_groups_full_path_check;

ALTER TABLE forge_connections
    DROP CONSTRAINT forge_connections_api_url_check,
    DROP CONSTRAINT forge_connections_owner_path_check,
    DROP CONSTRAINT forge_connections_credentials_ref_check,
    DROP CONSTRAINT forge_connections_credentials_fingerprint_check,
    DROP CONSTRAINT forge_connections_webhook_id_check;

ALTER TABLE service_environments
    DROP CONSTRAINT service_environments_service_check,
    DROP CONSTRAINT service_environments_environment_check;

ALTER TABLE service_deployments
    DROP CONSTRAINT service_deployments_service_check,
    DROP CONSTRAINT service_deployments_environment_check,
    DROP CONSTRAINT service_deployments_version_check,
    DROP CONSTRAINT service_deployments_commit_sha_check,
    DROP CONSTRAINT service_deployments_branch_check,
    DROP CONSTRAINT service_deployments_cluster_check,
    DROP CONSTRAINT service_deployments_namespace_check,
    DROP CONSTRAINT service_deployments_url_check,
    DROP CONSTRAINT service_deployments_deployed_by_check;

ALTER TABLE project_keys
    DROP CONSTRAINT project_keys_prefix_check;

ALTER TABLE project_events
    DROP CONSTRAINT project_events_idempotency_key_check;

ALTER TABLE branches
    DROP CONSTRAINT branches_name_check;

ALTER TABLE node_vars
    DROP CONSTRAINT node_vars_key_check,
    DROP CONSTRAINT node_vars_value_check;

ALTER TABLE nodes
    DROP CONSTRAINT nodes_slug_check,
    DROP CONSTRAINT nodes_name_check,
    DROP CONSTRAINT nodes_description_check,
    DROP CONSTRAINT nodes_repo_url_check,
    DROP CONSTRAINT nodes_default_branch_check;

ALTER TABLE secrets
    DROP CONSTRAINT secrets_value_ref_check,
    DROP CONSTRAINT secrets_fingerprint_check;

ALTER TABLE oauth_login_states
    DROP CONSTRAINT oauth_login_states_provider_check,
    DROP CONSTRAINT oauth_login_states_state_check,
    DROP CONSTRAINT oauth_login_states_nonce_check,
    DROP CONSTRAINT oauth_login_states_code_verifier_check,
    DROP CONSTRAINT oauth_login_states_next_check;

ALTER TABLE user_identities
    DROP CONSTRAINT user_identities_provider_check,
    DROP CONSTRAINT user_identities_subject_check,
    DROP CONSTRAINT user_identities_email_check;

ALTER TABLE personal_access_tokens
    DROP CONSTRAINT personal_access_tokens_prefix_check;

ALTER TABLE groups
    DROP CONSTRAINT groups_name_check;

ALTER TABLE users
    DROP CONSTRAINT users_email_check,
    DROP CONSTRAINT users_display_name_check,
    DROP CONSTRAINT users_password_hash_check;
