ALTER TABLE knowledge_sources RENAME COLUMN forge TO hosting;
ALTER TABLE knowledge_sources RENAME CONSTRAINT knowledge_sources_forge_check TO knowledge_sources_hosting_check;
