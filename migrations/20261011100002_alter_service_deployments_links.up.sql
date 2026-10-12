ALTER TABLE service_deployments
    ADD COLUMN links jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD CONSTRAINT service_deployments_links_check CHECK (jsonb_typeof(links) = 'array');
