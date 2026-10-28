ALTER TABLE skill_sources RENAME COLUMN forge TO hosting;
ALTER TABLE skill_sources RENAME CONSTRAINT skill_sources_forge_check TO skill_sources_hosting_check;
