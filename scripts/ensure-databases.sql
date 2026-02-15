-- =============================================================================
-- Create registry databases on an existing Postgres instance (e.g. platform).
-- =============================================================================
-- Run against the Postgres server (not a specific app DB). The connection user
-- must be allowed to create databases (e.g. platform user with CREATEDB).
--
--   psql -U morezero -d postgres -h localhost -f scripts/ensure-databases.sql
--
-- Or from registry dir on Windows (PowerShell):
--   $env:PGPASSWORD = "morezero"; psql -U morezero -d postgres -h localhost -f scripts/ensure-databases.sql
--
-- Creates: registry (for the app), registry_test (for go test -tags=integration).

\connect postgres

SELECT format('CREATE DATABASE %I', n)
FROM (VALUES ('registry'), ('registry_test')) AS t(n)
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = t.n);
\gexec

\connect registry
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

\connect registry_test
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
