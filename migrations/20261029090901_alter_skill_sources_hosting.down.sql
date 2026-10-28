ALTER TABLE skill_sources RENAME CONSTRAINT skill_sources_hosting_check TO skill_sources_forge_check;
ALTER TABLE skill_sources RENAME COLUMN hosting TO forge;
