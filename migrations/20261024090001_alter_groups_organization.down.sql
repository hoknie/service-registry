DROP TRIGGER nodes_keep_organization_groups ON nodes;
DROP FUNCTION nodes_keep_organization_groups();
DROP TRIGGER node_owners_group_organization ON node_owners;
DROP TRIGGER role_bindings_group_organization ON role_bindings;
DROP FUNCTION group_subject_in_organization();
DROP TRIGGER groups_organization_top_level ON groups;
DROP FUNCTION groups_organization_top_level();
DROP FUNCTION node_root(uuid);

DELETE FROM groups
WHERE organization_id IS NOT NULL;

DROP INDEX groups_organization_name_key;
DROP INDEX groups_name_lower_key;

ALTER TABLE groups
    DROP COLUMN organization_id;

CREATE UNIQUE INDEX groups_name_lower_key ON groups (lower(name));
