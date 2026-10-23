ALTER TABLE knowledge_patterns
    ADD COLUMN refresh text,
    ADD CONSTRAINT knowledge_patterns_refresh_check CHECK (refresh IN ('interval', 'event'));
