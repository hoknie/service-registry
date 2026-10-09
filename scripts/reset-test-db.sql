-- Drops schemas left behind by interrupted test runs (each test creates `t_<pid>_<n>_<nanos>`).
DO $$
DECLARE s text;
BEGIN
  FOR s IN SELECT nspname FROM pg_namespace WHERE nspname LIKE 't\_%' LOOP
    EXECUTE format('DROP SCHEMA %I CASCADE', s);
  END LOOP;
END $$;

-- The pgvector extension lives in public once, so parallel tests (one schema each) never race to create it.
CREATE EXTENSION IF NOT EXISTS vector SCHEMA public;
