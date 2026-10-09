ALTER TABLE nodes DROP CONSTRAINT nodes_forge_check;
ALTER TABLE nodes
    ADD CONSTRAINT nodes_forge_check CHECK (forge IN ('github', 'gitlab', 'forgejo'));
