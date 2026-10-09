ALTER TABLE service_deployments
    ADD COLUMN source text NOT NULL DEFAULT 'event';
ALTER TABLE service_deployments
    ADD CONSTRAINT service_deployments_source_check CHECK (source IN ('event', 'cluster'));
