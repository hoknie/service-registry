ALTER TABLE knowledge_settings
    ALTER COLUMN next_run_at DROP NOT NULL,
    ADD COLUMN triggered_at timestamptz;
