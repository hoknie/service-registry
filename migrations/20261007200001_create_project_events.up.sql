CREATE TABLE project_events
(
    id              uuid PRIMARY KEY,
    project_id      uuid        NOT NULL,
    key_id          uuid,
    type            text        NOT NULL,
    version         integer     NOT NULL,
    idempotency_key text        NOT NULL,
    occurred_at     timestamptz NOT NULL,
    received_at     timestamptz NOT NULL DEFAULT now(),
    payload         jsonb       NOT NULL,
    CONSTRAINT project_events_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT project_events_key_id_fkey FOREIGN KEY (key_id) REFERENCES project_keys (id) ON DELETE SET NULL,
    CONSTRAINT project_events_type_check CHECK (type IN ('service.deployed')),
    CONSTRAINT project_events_version_check CHECK (version >= 1),
    CONSTRAINT project_events_payload_check CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT project_events_idempotency_key UNIQUE (project_id, idempotency_key)
);

CREATE INDEX project_events_project_received_idx ON project_events (project_id, received_at DESC, id DESC);
CREATE INDEX project_events_received_at_idx ON project_events (received_at);
