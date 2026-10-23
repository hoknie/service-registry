ALTER TABLE knowledge_scans
    DROP CONSTRAINT knowledge_scans_trigger_check,
    ADD CONSTRAINT knowledge_scans_trigger_check CHECK (trigger IN ('schedule', 'manual', 'event'));
