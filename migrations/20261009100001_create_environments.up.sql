CREATE TABLE environments
(
    id         uuid PRIMARY KEY,
    key        text        NOT NULL,
    names      jsonb       NOT NULL,
    position   integer     NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT environments_key_key UNIQUE (key),
    CONSTRAINT environments_names_check CHECK (
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
    CONSTRAINT environments_position_check CHECK (position BETWEEN 0 AND 10000)
);

INSERT INTO environments (id, key, names, position)
VALUES ('01a12088-d200-7142-a41b-12d7df02380b', 'production',
        '{
          "en": "Production",
          "es": "Producción",
          "ru": "Продакшн",
          "zh": "生产"
        }', 10),
       ('01a12088-d201-7b3b-8f25-a47332504a32', 'staging',
        '{
          "en": "Staging",
          "es": "Preproducción",
          "ru": "Стейджинг",
          "zh": "预发布"
        }', 20),
       ('01a12088-d202-7612-85a7-7a803feed765', 'development',
        '{
          "en": "Development",
          "es": "Desarrollo",
          "ru": "Разработка",
          "zh": "开发"
        }', 30);
