ALTER TABLE service_deployments
    DROP CONSTRAINT service_deployments_links_check,
    DROP COLUMN links;
