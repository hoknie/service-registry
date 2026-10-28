ALTER TABLE nodes RENAME COLUMN forge TO hosting;
ALTER TABLE nodes RENAME CONSTRAINT nodes_forge_check TO nodes_hosting_check;
