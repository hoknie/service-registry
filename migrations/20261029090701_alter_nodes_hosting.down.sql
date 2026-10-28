ALTER TABLE nodes RENAME CONSTRAINT nodes_hosting_check TO nodes_forge_check;
ALTER TABLE nodes RENAME COLUMN hosting TO forge;
