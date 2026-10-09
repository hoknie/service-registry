CREATE TABLE service_environments
(
    id            uuid PRIMARY KEY,
    project_id    uuid        NOT NULL,
    service       text        NOT NULL,
    environment   text        NOT NULL,
    deployment_id uuid        NOT NULL,
    occurred_at   timestamptz NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT service_environments_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT service_environments_deployment_id_fkey FOREIGN KEY (deployment_id) REFERENCES service_deployments (id) ON DELETE CASCADE,
    CONSTRAINT service_environments_key UNIQUE (project_id, service, environment)
);

CREATE INDEX service_environments_deployment_id_idx ON service_environments (deployment_id);
