ALTER TABLE groups
    ADD COLUMN organization_id uuid,
    ADD CONSTRAINT groups_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE;

DROP INDEX groups_name_lower_key;

CREATE UNIQUE INDEX groups_name_lower_key ON groups (lower(name)) WHERE organization_id IS NULL;
CREATE UNIQUE INDEX groups_organization_name_key ON groups (organization_id, lower(name)) WHERE organization_id IS NOT NULL;

CREATE FUNCTION node_root(p_id uuid)
RETURNS uuid
LANGUAGE sql STABLE
AS $$
WITH RECURSIVE up AS (
    SELECT n.id, n.parent_id
    FROM nodes n
    WHERE n.id = p_id
    UNION ALL
    SELECT p.id, p.parent_id
    FROM up
    JOIN nodes p ON p.id = up.parent_id
)
SELECT id
FROM up
WHERE parent_id IS NULL
$$;

CREATE FUNCTION groups_organization_top_level()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM nodes n WHERE n.id = NEW.organization_id AND n.parent_id IS NOT NULL) THEN
        RAISE EXCEPTION 'group organization % is not a top-level node', NEW.organization_id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'groups_organization_id_check';
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER groups_organization_top_level
    BEFORE INSERT OR UPDATE OF organization_id ON groups
    FOR EACH ROW
    WHEN (NEW.organization_id IS NOT NULL)
    EXECUTE FUNCTION groups_organization_top_level();

CREATE FUNCTION group_subject_in_organization()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM groups g
        WHERE g.id = NEW.group_id
            AND g.organization_id IS NOT NULL
            AND g.organization_id IS DISTINCT FROM node_root(NEW.node_id)
    ) THEN
        RAISE EXCEPTION 'group % belongs to another organization', NEW.group_id
            USING ERRCODE = 'check_violation', CONSTRAINT = TG_TABLE_NAME || '_group_organization_check';
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER role_bindings_group_organization
    BEFORE INSERT OR UPDATE OF node_id, group_id ON role_bindings
    FOR EACH ROW
    WHEN (NEW.group_id IS NOT NULL)
    EXECUTE FUNCTION group_subject_in_organization();

CREATE TRIGGER node_owners_group_organization
    BEFORE INSERT OR UPDATE OF node_id, group_id ON node_owners
    FOR EACH ROW
    WHEN (NEW.group_id IS NOT NULL)
    EXECUTE FUNCTION group_subject_in_organization();

CREATE FUNCTION nodes_keep_organization_groups()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    v_root uuid;
BEGIN
    IF OLD.parent_id IS NULL AND EXISTS (SELECT 1 FROM groups g WHERE g.organization_id = NEW.id) THEN
        RAISE EXCEPTION 'organization % has groups', NEW.id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'nodes_organization_groups_check';
    END IF;
    v_root := CASE WHEN NEW.parent_id IS NULL THEN NEW.id ELSE node_root(NEW.parent_id) END;
    IF EXISTS (
        WITH RECURSIVE down AS (
            SELECT NEW.id AS id
            UNION ALL
            SELECT c.id
            FROM nodes c
            JOIN down d ON c.parent_id = d.id
        )
        SELECT 1
        FROM down d
        JOIN (
            SELECT node_id, group_id FROM role_bindings WHERE group_id IS NOT NULL
            UNION ALL
            SELECT node_id, group_id FROM node_owners WHERE group_id IS NOT NULL
        ) s ON s.node_id = d.id
        JOIN groups g ON g.id = s.group_id
        WHERE g.organization_id IS NOT NULL
            AND g.organization_id <> v_root
    ) THEN
        RAISE EXCEPTION 'subtree of % carries roles of another organization''s groups', NEW.id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'nodes_group_organization_check';
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER nodes_keep_organization_groups
    BEFORE UPDATE OF parent_id ON nodes
    FOR EACH ROW
    WHEN (OLD.parent_id IS DISTINCT FROM NEW.parent_id)
    EXECUTE FUNCTION nodes_keep_organization_groups();
