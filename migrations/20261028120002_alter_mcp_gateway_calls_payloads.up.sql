ALTER TABLE mcp_gateway_calls
    ADD COLUMN request text,
    ADD COLUMN response text,
    ADD COLUMN payload_truncated boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT mcp_gateway_calls_request_check CHECK (octet_length(request) <= 16384),
    ADD CONSTRAINT mcp_gateway_calls_response_check CHECK (octet_length(response) <= 16384);
