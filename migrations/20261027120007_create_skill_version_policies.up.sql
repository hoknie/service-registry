CREATE TABLE skill_version_policies
(
    id              uuid PRIMARY KEY,
    skill_id        uuid,
    organization_id uuid,
    version_range   text,
    prerelease      boolean,
    excluded        text[],
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT skill_version_policies_skill_id_fkey FOREIGN KEY (skill_id) REFERENCES skills (id) ON DELETE CASCADE,
    CONSTRAINT skill_version_policies_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT skill_version_policies_owner_check CHECK (num_nonnulls(skill_id, organization_id) = 1),
    CONSTRAINT skill_version_policies_version_range_check CHECK (char_length(version_range) BETWEEN 1 AND 200),
    CONSTRAINT skill_version_policies_excluded_check CHECK (cardinality(excluded) <= 50)
);

CREATE UNIQUE INDEX skill_version_policies_skill_idx ON skill_version_policies (skill_id) WHERE skill_id IS NOT NULL;

CREATE UNIQUE INDEX skill_version_policies_organization_idx ON skill_version_policies (organization_id)
    WHERE organization_id IS NOT NULL;
