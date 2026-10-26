ALTER TABLE group_permissions
    DROP CONSTRAINT group_permissions_section_check,
    ADD CONSTRAINT group_permissions_section_check CHECK (section IN ('users', 'groups', 'tokens', 'clusters', 'environments', 'link_kinds', 'secrets', 'scans', 'catalog', 'publication'));
