DO
$$
BEGIN
    IF
EXISTS (SELECT 1 FROM service_deployments WHERE source = 'cluster') THEN
        RAISE EXCEPTION 'service_deployments has deployments with source = cluster';
END IF;
END $$;
ALTER TABLE service_deployments DROP COLUMN source;
