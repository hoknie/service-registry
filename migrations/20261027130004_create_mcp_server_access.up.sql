CREATE TABLE mcp_server_access
(
    server_id uuid NOT NULL,
    group_id  uuid,
    user_id   uuid,
    CONSTRAINT mcp_server_access_server_id_fkey FOREIGN KEY (server_id) REFERENCES mcp_servers (id) ON DELETE CASCADE,
    CONSTRAINT mcp_server_access_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
    CONSTRAINT mcp_server_access_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT mcp_server_access_subject_check CHECK (num_nonnulls(group_id, user_id) = 1)
);

CREATE UNIQUE INDEX mcp_server_access_group_idx ON mcp_server_access (server_id, group_id) WHERE group_id IS NOT NULL;

CREATE UNIQUE INDEX mcp_server_access_user_idx ON mcp_server_access (server_id, user_id) WHERE user_id IS NOT NULL;

CREATE INDEX mcp_server_access_group_id_idx ON mcp_server_access (group_id) WHERE group_id IS NOT NULL;

CREATE INDEX mcp_server_access_user_id_idx ON mcp_server_access (user_id) WHERE user_id IS NOT NULL;
