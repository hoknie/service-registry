CREATE TABLE cluster_workloads
(
    id              uuid PRIMARY KEY,
    cluster_id      uuid        NOT NULL,
    uid             text        NOT NULL,
    namespace       text        NOT NULL,
    kind            text        NOT NULL,
    name            text        NOT NULL,
    project_id      uuid,
    reason          text,
    annotation      text,
    service         text,
    environment     text,
    branch          text,
    version         text,
    images          jsonb       NOT NULL DEFAULT '[]'::jsonb,
    state           jsonb       NOT NULL DEFAULT '{}'::jsonb,
    pending_version text,
    pending_since   timestamptz,
    observed_at     timestamptz NOT NULL DEFAULT now(),
    gone_at         timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT cluster_workloads_cluster_id_fkey FOREIGN KEY (cluster_id) REFERENCES clusters (id) ON DELETE CASCADE,
    CONSTRAINT cluster_workloads_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT cluster_workloads_uid_key UNIQUE (cluster_id, uid),
    CONSTRAINT cluster_workloads_kind_check CHECK (kind IN ('Deployment', 'StatefulSet', 'DaemonSet', 'CronJob')),
    CONSTRAINT cluster_workloads_reason_check
        CHECK (reason IN ('no_annotation', 'project_not_found', 'invalid_service', 'invalid_environment')),
    CONSTRAINT cluster_workloads_match_check CHECK ((project_id IS NULL) = (reason IS NOT NULL))
);

CREATE INDEX cluster_workloads_triple_idx ON cluster_workloads (project_id, service, environment);
CREATE INDEX cluster_workloads_gone_idx ON cluster_workloads (gone_at) WHERE gone_at IS NOT NULL;
