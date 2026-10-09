CREATE TABLE link_kinds
(
    id         uuid PRIMARY KEY,
    key        text        NOT NULL,
    names      jsonb       NOT NULL,
    icon       text        NOT NULL DEFAULT 'link',
    position   integer     NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT link_kinds_key_key UNIQUE (key),
    CONSTRAINT link_kinds_names_check CHECK (
        jsonb_typeof(names) = 'object'
            AND names ?& ARRAY['en', 'es', 'ru', 'zh']
    AND
    names
    -
    ARRAY[
    'en', 'es', 'ru', 'zh'] =
    '{}'
    :
    :
    jsonb
    AND
    jsonb_typeof
(
    names
    ->
    'en'
) = 'string' AND jsonb_typeof
(
    names
    ->
    'es'
) = 'string'
    AND jsonb_typeof
(
    names
    ->
    'ru'
) = 'string' AND jsonb_typeof
(
    names
    ->
    'zh'
) = 'string'
    ) ,
    CONSTRAINT link_kinds_icon_check
        CHECK (icon IN ('link', 'logs', 'dashboard', 'errors', 'alerts', 'traces', 'runbook', 'docs')),
    CONSTRAINT link_kinds_position_check CHECK (position BETWEEN 0 AND 10000)
);

INSERT INTO link_kinds (id, key, names, icon, position)
VALUES ('01a11b62-7600-77c7-a462-520a4e923dbc', 'logs',
        '{
          "en": "Logs",
          "es": "Registros",
          "ru": "Логи",
          "zh": "日志"
        }', 'logs', 10),
       ('01a11b62-7601-78d0-82b5-dcee8ba7d4ac', 'grafana',
        '{
          "en": "Grafana",
          "es": "Grafana",
          "ru": "Grafana",
          "zh": "Grafana"
        }', 'dashboard', 20),
       ('01a11b62-7602-70ea-a6b5-ab5b1c95ba1b', 'sentry',
        '{
          "en": "Sentry",
          "es": "Sentry",
          "ru": "Sentry",
          "zh": "Sentry"
        }', 'errors', 30),
       ('01a11b62-7603-71c8-9e3f-99152631d429', 'alertmanager',
        '{
          "en": "Alerts",
          "es": "Alertas",
          "ru": "Алерты",
          "zh": "告警"
        }', 'alerts', 40),
       ('01a11b62-7604-79e0-ae6d-d16082ae232b', 'traces',
        '{
          "en": "Traces",
          "es": "Trazas",
          "ru": "Трейсы",
          "zh": "链路追踪"
        }', 'traces', 50),
       ('01a11b62-7605-7719-a1f2-ecfe46f35289', 'runbook',
        '{
          "en": "Runbook",
          "es": "Runbook",
          "ru": "Ранбук",
          "zh": "运维手册"
        }', 'runbook', 60),
       ('01a11b62-7606-7efb-ad16-6c20f5e96077', 'docs',
        '{
          "en": "Docs",
          "es": "Documentación",
          "ru": "Документация",
          "zh": "文档"
        }', 'docs', 70);
