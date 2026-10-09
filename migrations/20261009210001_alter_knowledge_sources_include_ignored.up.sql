ALTER TABLE knowledge_sources
    ADD COLUMN include_ignored boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT knowledge_sources_include_ignored_check CHECK (NOT include_ignored OR (kind = 'local_git' AND working_tree));
