ALTER TABLE forge_connections
    ADD COLUMN branch_include text[] NOT NULL DEFAULT '{}';
