CREATE TABLE mcp_builtin_settings
(
    id             boolean PRIMARY KEY  DEFAULT true,
    disabled_tools text[]      NOT NULL DEFAULT '{}',
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mcp_builtin_settings_id_check CHECK (id),
    CONSTRAINT mcp_builtin_settings_disabled_tools_check CHECK (cardinality(disabled_tools) <= 100)
);
