ALTER TABLE knowledge_sources RENAME CONSTRAINT knowledge_sources_hosting_check TO knowledge_sources_forge_check;
ALTER TABLE knowledge_sources RENAME COLUMN hosting TO forge;
