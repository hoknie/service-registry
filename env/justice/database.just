# Apply pending migrations (DATABASE_URL)
[group('database')]
db-migrate:
    {{ GO }} run ./cmd/svc-registry db:migrate

# Revert the last COUNT applied migrations: `just db-rollback [COUNT]`
[group('database')]
db-rollback COUNT='1':
    {{ GO }} run ./cmd/svc-registry db:rollback --count {{ COUNT }}

# Wipe the dev database (pgsql on :5440: drop + recreate schema public), then migrate
[group('database')]
db-reset:
    docker compose exec -T pgsql psql -U registry -d registry -v ON_ERROR_STOP=1 -q -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'
    {{ GO }} run ./cmd/svc-registry db:migrate

# Create an empty reversible migration pair: `just new-migration create_projects`
[group('database')]
new-migration NAME:
    env/scripts/new-migration.sh {{ NAME }}
