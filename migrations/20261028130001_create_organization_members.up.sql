CREATE TABLE organization_members
(
    organization_id uuid        NOT NULL,
    user_id         uuid        NOT NULL,
    source          text        NOT NULL,
    added_by        uuid,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT organization_members_pkey PRIMARY KEY (organization_id, user_id),
    CONSTRAINT organization_members_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT organization_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT organization_members_added_by_fkey FOREIGN KEY (added_by) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT organization_members_source_check CHECK (source IN ('manual', 'invitation', 'role', 'owner', 'group'))
);

CREATE INDEX organization_members_user_idx ON organization_members (user_id);

CREATE FUNCTION organization_members_top_level()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM nodes WHERE id = NEW.organization_id AND parent_id IS NOT NULL) THEN
        RAISE EXCEPTION 'organization members belong to top-level nodes only' USING ERRCODE = 'check_violation',
            CONSTRAINT = 'organization_members_top_level_check';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER organization_members_top_level
BEFORE INSERT OR UPDATE OF organization_id ON organization_members
FOR EACH ROW EXECUTE FUNCTION organization_members_top_level();

CREATE FUNCTION organization_member_join(p_node uuid, p_user uuid, p_source text)
RETURNS void
LANGUAGE sql
AS $$
INSERT INTO organization_members (organization_id, user_id, source)
SELECT node_root(p_node), p_user, p_source
WHERE node_root(p_node) IS NOT NULL
ON CONFLICT DO NOTHING
$$;

CREATE FUNCTION organization_members_from_binding()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.user_id IS NOT NULL THEN
        PERFORM organization_member_join(NEW.node_id, NEW.user_id, 'role');
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER organization_members_from_binding
AFTER INSERT OR UPDATE OF node_id, user_id ON role_bindings
FOR EACH ROW EXECUTE FUNCTION organization_members_from_binding();

CREATE FUNCTION organization_members_from_owner()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.user_id IS NOT NULL THEN
        PERFORM organization_member_join(NEW.node_id, NEW.user_id, 'owner');
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER organization_members_from_owner
AFTER INSERT OR UPDATE OF node_id, user_id ON node_owners
FOR EACH ROW EXECUTE FUNCTION organization_members_from_owner();

CREATE FUNCTION organization_members_from_group()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO organization_members (organization_id, user_id, source)
    SELECT g.organization_id, NEW.user_id, 'group'
    FROM groups g
    WHERE g.id = NEW.group_id
        AND g.organization_id IS NOT NULL
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;

CREATE TRIGGER organization_members_from_group
AFTER INSERT ON group_members
FOR EACH ROW EXECUTE FUNCTION organization_members_from_group();

CREATE FUNCTION organization_members_from_move()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.parent_id IS NULL AND NEW.parent_id IS NOT NULL THEN
        DELETE FROM organization_members WHERE organization_id = NEW.id;
    END IF;
    IF NEW.parent_id IS NULL THEN
        RETURN NULL;
    END IF;
    WITH RECURSIVE down AS (
        SELECT NEW.id AS id
        UNION ALL
        SELECT c.id
        FROM nodes c
        JOIN down ON c.parent_id = down.id
    ),
    people AS (
        SELECT b.user_id, 'role' AS source
        FROM role_bindings b
        WHERE b.node_id IN (SELECT id FROM down) AND b.user_id IS NOT NULL
        UNION ALL
        SELECT o.user_id, 'owner'
        FROM node_owners o
        WHERE o.node_id IN (SELECT id FROM down) AND o.user_id IS NOT NULL
    )
    INSERT INTO organization_members (organization_id, user_id, source)
    SELECT DISTINCT ON (user_id) node_root(NEW.id), user_id, source
    FROM people
    ORDER BY user_id, source DESC
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;

CREATE TRIGGER organization_members_from_move
AFTER UPDATE OF parent_id ON nodes
FOR EACH ROW
WHEN (OLD.parent_id IS DISTINCT FROM NEW.parent_id)
EXECUTE FUNCTION organization_members_from_move();

CREATE FUNCTION organization_admins(p_org uuid)
RETURNS bigint
LANGUAGE sql STABLE
AS $$
SELECT count(DISTINCT u.id)
FROM users u
WHERE NOT u.is_superadmin
    AND (
        EXISTS (SELECT 1 FROM role_bindings b WHERE b.node_id = p_org AND b.user_id = u.id AND b.role = 'admin')
        OR EXISTS (
            SELECT 1
            FROM role_bindings b
            JOIN group_members m ON m.group_id = b.group_id
            WHERE b.node_id = p_org AND b.role = 'admin' AND m.user_id = u.id
        )
    )
$$;

CREATE FUNCTION remove_organization_member(p_org uuid, p_user uuid)
RETURNS boolean
LANGUAGE plpgsql
AS $$
DECLARE
    removed boolean;
BEGIN
    WITH RECURSIVE down AS (
        SELECT p_org AS id
        UNION ALL
        SELECT c.id
        FROM nodes c
        JOIN down ON c.parent_id = down.id
    ),
    roles AS (
        DELETE FROM role_bindings
        WHERE user_id = p_user AND node_id IN (SELECT id FROM down)
    ),
    owned AS (
        DELETE FROM node_owners
        WHERE user_id = p_user AND node_id IN (SELECT id FROM down)
    ),
    grouped AS (
        DELETE FROM group_members m
        USING groups g
        WHERE m.group_id = g.id AND g.organization_id = p_org AND m.user_id = p_user
    )
    DELETE FROM organization_members
    WHERE organization_id = p_org AND user_id = p_user;
    removed := FOUND;
    RETURN removed;
END;
$$;

INSERT INTO organization_members (organization_id, user_id, source, created_at)
SELECT DISTINCT ON (org, user_id) org, user_id, source, created_at
FROM (
    SELECT node_root(b.node_id) AS org, b.user_id, 'role' AS source, b.created_at
    FROM role_bindings b
    WHERE b.user_id IS NOT NULL
    UNION ALL
    SELECT node_root(o.node_id), o.user_id, 'owner', o.created_at
    FROM node_owners o
    WHERE o.user_id IS NOT NULL
    UNION ALL
    SELECT g.organization_id, m.user_id, 'group', m.created_at
    FROM group_members m
    JOIN groups g ON g.id = m.group_id
    WHERE g.organization_id IS NOT NULL
) found
WHERE org IS NOT NULL
ORDER BY org, user_id, created_at
ON CONFLICT DO NOTHING;
