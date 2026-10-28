CREATE TABLE mcp_gateway_calls
(
    id          uuid PRIMARY KEY,
    server_id   uuid        NOT NULL,
    tool        text        NOT NULL,
    user_id     uuid,
    token_id    uuid,
    grant_id    uuid,
    duration_ms integer     NOT NULL,
    outcome     text        NOT NULL,
    code        text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mcp_gateway_calls_server_id_fkey FOREIGN KEY (server_id) REFERENCES mcp_servers (id) ON DELETE CASCADE,
    CONSTRAINT mcp_gateway_calls_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT mcp_gateway_calls_token_id_fkey FOREIGN KEY (token_id) REFERENCES personal_access_tokens (id) ON DELETE SET NULL,
    CONSTRAINT mcp_gateway_calls_grant_id_fkey FOREIGN KEY (grant_id) REFERENCES oauth_grants (id) ON DELETE SET NULL,
    CONSTRAINT mcp_gateway_calls_tool_check CHECK (char_length(tool) BETWEEN 1 AND 128),
    CONSTRAINT mcp_gateway_calls_duration_ms_check CHECK (duration_ms >= 0),
    CONSTRAINT mcp_gateway_calls_outcome_check CHECK (outcome IN ('ok', 'tool_error', 'failed')),
    CONSTRAINT mcp_gateway_calls_code_check CHECK (code ~ '^[a-z][a-z_.]{0,63}$')
);

CREATE INDEX mcp_gateway_calls_server_idx ON mcp_gateway_calls (server_id, created_at DESC);

CREATE INDEX mcp_gateway_calls_created_idx ON mcp_gateway_calls (created_at);
