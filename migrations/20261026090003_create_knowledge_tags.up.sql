CREATE TABLE knowledge_tags
(
    project_id    uuid        NOT NULL,
    name          text        NOT NULL,
    commit_sha    text        NOT NULL,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    release       boolean     NOT NULL DEFAULT false,
    CONSTRAINT knowledge_tags_pkey PRIMARY KEY (project_id, name),
    CONSTRAINT knowledge_tags_project_id_fkey FOREIGN KEY (project_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT knowledge_tags_name_check CHECK (char_length(name) BETWEEN 1 AND 255),
    CONSTRAINT knowledge_tags_commit_sha_check CHECK (char_length(commit_sha) BETWEEN 1 AND 255)
);

CREATE UNIQUE INDEX knowledge_tags_release_idx ON knowledge_tags (project_id) WHERE release;
