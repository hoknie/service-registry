CREATE TABLE skill_organization_sets
(
    organization_id uuid PRIMARY KEY,
    mode            text        NOT NULL DEFAULT 'all',
    folder_ids      uuid[]      NOT NULL DEFAULT '{}',
    skill_ids       uuid[]      NOT NULL DEFAULT '{}',
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT skill_organization_sets_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT skill_organization_sets_mode_check CHECK (mode IN ('all', 'selected')),
    CONSTRAINT skill_organization_sets_items_check CHECK (cardinality(folder_ids) + cardinality(skill_ids) <= 500)
);

CREATE INDEX skill_organization_sets_folder_ids_idx ON skill_organization_sets USING gin (folder_ids);

CREATE INDEX skill_organization_sets_skill_ids_idx ON skill_organization_sets USING gin (skill_ids);

CREATE FUNCTION skill_folder_ancestors(folder uuid) RETURNS uuid[]
    LANGUAGE sql
    STABLE
AS
$$
WITH RECURSIVE chain AS (SELECT id, parent_id
                         FROM skill_folders
                         WHERE id = folder
                         UNION ALL
                         SELECT f.id, f.parent_id
                         FROM skill_folders f
                                  JOIN chain c ON f.id = c.parent_id)
SELECT COALESCE(array_agg(id), '{}')
FROM chain
$$;

CREATE FUNCTION skill_in_sets(skill uuid, skill_org uuid, folder uuid, orgs uuid[]) RETURNS boolean
    LANGUAGE sql
    STABLE
AS
$$
SELECT orgs IS NULL
           OR skill_org = ANY (orgs)
           OR (skill_org IS NULL AND EXISTS (SELECT 1
                                             FROM unnest(orgs) AS o(id)
                                                      LEFT JOIN skill_organization_sets s ON s.organization_id = o.id
                                             WHERE s.organization_id IS NULL
                                                OR s.mode = 'all'
                                                OR skill = ANY (s.skill_ids)
                                                OR s.folder_ids && skill_folder_ancestors(folder)))
$$;

CREATE FUNCTION skill_sets_forget_folder() RETURNS trigger
    LANGUAGE plpgsql
AS
$$
BEGIN
    UPDATE skill_organization_sets
    SET folder_ids = array_remove(folder_ids, OLD.id)
    WHERE folder_ids @> ARRAY [OLD.id];
    RETURN OLD;
END
$$;

CREATE FUNCTION skill_sets_forget_skill() RETURNS trigger
    LANGUAGE plpgsql
AS
$$
BEGIN
    UPDATE skill_organization_sets
    SET skill_ids = array_remove(skill_ids, OLD.id)
    WHERE skill_ids @> ARRAY [OLD.id];
    RETURN OLD;
END
$$;

CREATE TRIGGER skill_folders_forget_in_sets
    AFTER DELETE
    ON skill_folders
    FOR EACH ROW
EXECUTE FUNCTION skill_sets_forget_folder();

CREATE TRIGGER skills_forget_in_sets
    AFTER DELETE
    ON skills
    FOR EACH ROW
EXECUTE FUNCTION skill_sets_forget_skill();
