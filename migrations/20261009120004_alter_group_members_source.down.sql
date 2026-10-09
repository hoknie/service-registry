DO
$$
BEGIN
    IF
EXISTS (SELECT 1 FROM group_members WHERE source <> 'manual') THEN
        RAISE EXCEPTION 'group_members has memberships managed by a provider';
END IF;
END $$;
ALTER TABLE group_members DROP COLUMN source;
