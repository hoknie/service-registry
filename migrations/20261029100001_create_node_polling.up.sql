CREATE TABLE node_polling
(
    node_id       uuid PRIMARY KEY,
    interval_secs integer,
    timeout_secs  integer,
    mode          text,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT node_polling_node_id_fkey FOREIGN KEY (node_id) REFERENCES nodes (id) ON DELETE CASCADE,
    CONSTRAINT node_polling_interval_secs_check CHECK (interval_secs BETWEEN 60 AND 86400),
    CONSTRAINT node_polling_timeout_secs_check CHECK (timeout_secs BETWEEN 1 AND 600),
    CONSTRAINT node_polling_mode_check CHECK (mode IN ('on', 'off', 'published'))
);
