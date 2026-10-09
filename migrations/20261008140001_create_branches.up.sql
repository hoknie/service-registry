CREATE TABLE branches
(
    id               uuid PRIMARY KEY,
    project_id       uuid        NOT NULL,
    name             text        NOT NULL,
    head_sha         text,
    is_default       boolean     NOT NULL DEFAULT false,
    protected        boolean,
    sources          text[]      NOT NULL,
    pinned           boolean     NOT NULL DEFAULT false,
    first_seen_at    timestamptz NOT NULL DEFAULT now(),
    last_activity_at timestamptz NOT NULL DEFAULT now(),
    gone_at          timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT branches_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT branches_name_key UNIQUE (project_id, name),
    CONSTRAINT branches_head_sha_check CHECK (head_sha ~ '^[0-9a-f]{7,64}$'
) ,
    CONSTRAINT branches_sources_check
        CHECK (cardinality(sources) > 0 AND sources <@ ARRAY['forge', 'ingest', 'manual']::text[])
);

CREATE UNIQUE INDEX branches_default_key ON branches (project_id) WHERE is_default;
CREATE INDEX branches_activity_idx ON branches (project_id, last_activity_at DESC);

INSERT INTO branches (id, project_id, name, is_default, sources)
SELECT encode(set_bit(set_bit(overlay(uuid_send(gen_random_uuid()) PLACING substring(int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3)
           FROM 1 FOR 6), 52, 1), 53, 1), 'hex')
    ::uuid, n.id
     , n.default_branch
     , true
     , ARRAY['manual']
FROM nodes n
WHERE n.kind = 'project'
  AND n.default_branch IS NOT NULL;
