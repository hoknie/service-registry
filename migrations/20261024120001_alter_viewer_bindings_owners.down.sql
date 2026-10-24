CREATE OR REPLACE FUNCTION viewer_bindings(p_user uuid, p_roots uuid[], p_admin boolean)
RETURNS TABLE (node_id uuid, role text)
LANGUAGE sql STABLE
AS $$
WITH RECURSIVE own AS (
    SELECT b.node_id, b.role
    FROM role_bindings b
    WHERE b.user_id = p_user
    UNION ALL
    SELECT b.node_id, b.role
    FROM role_bindings b
    JOIN group_members gm ON gm.group_id = b.group_id
    WHERE gm.user_id = p_user
    UNION ALL
    SELECT n.id, 'viewer'
    FROM nodes n
    WHERE p_user IS NOT NULL
        AND n.parent_id IS NULL
        AND n.visibility = 'open'
),
above AS (
    SELECT n.id AS root, n.id, n.parent_id
    FROM nodes n
    WHERE p_roots IS NOT NULL
        AND n.id = ANY (p_roots)
    UNION ALL
    SELECT a.root, p.id, p.parent_id
    FROM above a
    JOIN nodes p ON p.id = a.parent_id
),
below AS (
    SELECT o.node_id AS bound, o.role, n.parent_id
    FROM own o
    JOIN nodes n ON n.id = o.node_id
    WHERE p_roots IS NOT NULL
    UNION ALL
    SELECT b.bound, b.role, p.parent_id
    FROM below b
    JOIN nodes p ON p.id = b.parent_id
)
SELECT o.node_id, o.role
FROM own o
WHERE p_roots IS NULL
UNION ALL
SELECT a.root, o.role
FROM above a
JOIN own o ON o.node_id = a.id
UNION ALL
SELECT b.bound, b.role
FROM below b
WHERE b.parent_id = ANY (p_roots)
UNION ALL
SELECT n.id, 'admin'
FROM nodes n
WHERE p_admin
    AND (n.id = ANY (p_roots) OR (p_roots IS NULL AND n.parent_id IS NULL))
$$;
