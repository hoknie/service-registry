CREATE TABLE service_deployments
(
    id             uuid PRIMARY KEY,
    project_id     uuid        NOT NULL,
    event_id       uuid,
    service        text        NOT NULL,
    environment    text        NOT NULL,
    version        text        NOT NULL,
    commit_sha     text,
    branch         text,
    cluster        text,
    namespace      text,
    url            text,
    deployed_by    text,
    metadata       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    occurred_at    timestamptz NOT NULL,
    became_current boolean     NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT service_deployments_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT service_deployments_event_id_fkey FOREIGN KEY (event_id) REFERENCES project_events (id) ON DELETE SET NULL,
    CONSTRAINT service_deployments_event_id_key UNIQUE (event_id),
    CONSTRAINT service_deployments_metadata_check CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX service_deployments_project_occurred_idx ON service_deployments (project_id, occurred_at DESC, id DESC);
CREATE INDEX service_deployments_project_env_idx ON service_deployments (project_id, environment, occurred_at DESC);
