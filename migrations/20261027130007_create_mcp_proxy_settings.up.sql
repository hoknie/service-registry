CREATE TABLE mcp_proxy_settings
(
    id                    uuid PRIMARY KEY,
    organization_id       uuid,
    folder_id             uuid,
    server_id             uuid,
    mode                  text        NOT NULL,
    url                   text,
    no_proxy              text,
    credentials_secret_id uuid,
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mcp_proxy_settings_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT mcp_proxy_settings_folder_id_fkey FOREIGN KEY (folder_id) REFERENCES mcp_folders (id) ON DELETE CASCADE,
    CONSTRAINT mcp_proxy_settings_server_id_fkey FOREIGN KEY (server_id) REFERENCES mcp_servers (id) ON DELETE CASCADE,
    CONSTRAINT mcp_proxy_settings_credentials_secret_id_fkey FOREIGN KEY (credentials_secret_id) REFERENCES secrets (id),
    CONSTRAINT mcp_proxy_settings_owner_check CHECK (num_nonnulls(organization_id, folder_id, server_id) = 1),
    CONSTRAINT mcp_proxy_settings_organization_id_key UNIQUE (organization_id),
    CONSTRAINT mcp_proxy_settings_folder_id_key UNIQUE (folder_id),
    CONSTRAINT mcp_proxy_settings_server_id_key UNIQUE (server_id),
    CONSTRAINT mcp_proxy_settings_mode_check CHECK (mode IN ('direct', 'environment', 'proxy')),
    CONSTRAINT mcp_proxy_settings_url_check CHECK (url ~ '^https?://' AND char_length(url) <= 2000),
    CONSTRAINT mcp_proxy_settings_no_proxy_check CHECK (char_length(no_proxy) <= 2000),
    CONSTRAINT mcp_proxy_settings_fields_check CHECK (
        (mode = 'proxy' AND url IS NOT NULL)
            OR (mode <> 'proxy' AND url IS NULL AND no_proxy IS NULL AND credentials_secret_id IS NULL))
);

CREATE INDEX mcp_proxy_settings_secret_idx ON mcp_proxy_settings (credentials_secret_id) WHERE credentials_secret_id IS NOT NULL;
