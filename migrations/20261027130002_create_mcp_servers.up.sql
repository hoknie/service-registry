CREATE TABLE mcp_servers
(
    id                    uuid PRIMARY KEY,
    folder_id             uuid,
    organization_id       uuid,
    slug                  text        NOT NULL,
    title                 text        NOT NULL,
    description           text        NOT NULL DEFAULT '',
    tags                  text[]      NOT NULL DEFAULT '{}',
    author                text        NOT NULL DEFAULT '',
    icon_url              text,
    url                   text        NOT NULL,
    transport             text        NOT NULL DEFAULT 'streamable',
    auth_kind             text        NOT NULL DEFAULT 'none',
    auth_header           text,
    credentials_secret_id uuid,
    status                text        NOT NULL DEFAULT 'draft',
    disabled_tools        text[]      NOT NULL DEFAULT '{}',
    everyone              boolean     NOT NULL DEFAULT true,
    calls                 bigint      NOT NULL DEFAULT 0,
    created_by            uuid,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mcp_servers_folder_id_fkey FOREIGN KEY (folder_id) REFERENCES mcp_folders (id) ON DELETE CASCADE,
    CONSTRAINT mcp_servers_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT mcp_servers_credentials_secret_id_fkey FOREIGN KEY (credentials_secret_id) REFERENCES secrets (id),
    CONSTRAINT mcp_servers_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT mcp_servers_slug_key UNIQUE (slug),
    CONSTRAINT mcp_servers_slug_check CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,31}$'),
    CONSTRAINT mcp_servers_title_check CHECK (char_length(title) BETWEEN 1 AND 100),
    CONSTRAINT mcp_servers_description_check CHECK (char_length(description) <= 4000),
    CONSTRAINT mcp_servers_tags_check CHECK (cardinality(tags) <= 20),
    CONSTRAINT mcp_servers_author_check CHECK (char_length(author) <= 100),
    CONSTRAINT mcp_servers_icon_url_check CHECK (icon_url ~ '^https://' AND char_length(icon_url) <= 2000),
    CONSTRAINT mcp_servers_url_check CHECK (url ~ '^https?://' AND char_length(url) <= 2000),
    CONSTRAINT mcp_servers_transport_check CHECK (transport IN ('streamable', 'sse')),
    CONSTRAINT mcp_servers_auth_kind_check CHECK (auth_kind IN ('none', 'bearer', 'header')),
    CONSTRAINT mcp_servers_auth_header_check CHECK (auth_header ~ '^[!#$%&''*+.^_`|~0-9A-Za-z-]{1,100}$'),
    CONSTRAINT mcp_servers_auth_check CHECK (
        (auth_kind = 'none' AND auth_header IS NULL AND credentials_secret_id IS NULL)
            OR (auth_kind = 'bearer' AND auth_header IS NULL AND credentials_secret_id IS NOT NULL)
            OR (auth_kind = 'header' AND auth_header IS NOT NULL AND credentials_secret_id IS NOT NULL)),
    CONSTRAINT mcp_servers_status_check CHECK (status IN ('draft', 'published', 'disabled')),
    CONSTRAINT mcp_servers_disabled_tools_check CHECK (cardinality(disabled_tools) <= 500),
    CONSTRAINT mcp_servers_calls_check CHECK (calls >= 0)
);

CREATE INDEX mcp_servers_folder_idx ON mcp_servers (folder_id);

CREATE INDEX mcp_servers_organization_idx ON mcp_servers (organization_id);

CREATE INDEX mcp_servers_secret_idx ON mcp_servers (credentials_secret_id) WHERE credentials_secret_id IS NOT NULL;

CREATE INDEX mcp_servers_tags_idx ON mcp_servers USING gin (tags);
