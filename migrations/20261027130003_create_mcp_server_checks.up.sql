CREATE TABLE mcp_server_checks
(
    server_id     uuid PRIMARY KEY,
    next_check_at timestamptz NOT NULL DEFAULT now(),
    lease_until   timestamptz,
    checked_at    timestamptz,
    outcome       text,
    code          text,
    detail        text,
    duration_ms   integer,
    truncated     boolean     NOT NULL DEFAULT false,
    snapshot_at   timestamptz,
    server_info   jsonb,
    tools         jsonb,
    CONSTRAINT mcp_server_checks_server_id_fkey FOREIGN KEY (server_id) REFERENCES mcp_servers (id) ON DELETE CASCADE,
    CONSTRAINT mcp_server_checks_outcome_check CHECK (outcome IN ('ok', 'failed')),
    CONSTRAINT mcp_server_checks_code_check CHECK (code ~ '^[a-z][a-z_.]{0,63}$'),
    CONSTRAINT mcp_server_checks_detail_check CHECK (char_length(detail) <= 500),
    CONSTRAINT mcp_server_checks_duration_ms_check CHECK (duration_ms >= 0),
    CONSTRAINT mcp_server_checks_server_info_check CHECK (jsonb_typeof(server_info) = 'object'),
    CONSTRAINT mcp_server_checks_tools_check CHECK (jsonb_typeof(tools) = 'array'),
    CONSTRAINT mcp_server_checks_snapshot_check CHECK ((snapshot_at IS NULL) = (tools IS NULL))
);

CREATE INDEX mcp_server_checks_due_idx ON mcp_server_checks (next_check_at);
