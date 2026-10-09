CREATE TABLE link_templates
(
    id         uuid PRIMARY KEY,
    node_id    uuid        NOT NULL,
    kind_id    uuid        NOT NULL,
    link_key   text        NOT NULL,
    template   text,
    disabled   boolean     NOT NULL DEFAULT false,
    position   integer     NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT link_templates_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT link_templates_kind_id_fkey FOREIGN KEY (kind_id) REFERENCES link_kinds (id) ON DELETE RESTRICT,
    CONSTRAINT link_templates_link_key_key UNIQUE (node_id, link_key),
    CONSTRAINT link_templates_template_check CHECK ((template IS NULL) = disabled),
    CONSTRAINT link_templates_position_check CHECK (position BETWEEN 0 AND 10000)
);

CREATE INDEX link_templates_kind_idx ON link_templates (kind_id);
