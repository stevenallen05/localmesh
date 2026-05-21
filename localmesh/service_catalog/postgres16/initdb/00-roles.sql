-- LocalMesh role provisioning. Runs BEFORE 01-pg_tracing.sql /
-- 02-pg_stat_statements.sql (numeric prefix orders initdb scripts).
--
-- Two cert-bound roles in scope: `server` (the application workload) and
-- `postgres-exporter` (observability scraper). The `app` role that the
-- legacy POSTGRES_USER env created is retired in the docker-compose env
-- block; this script doesn't try to drop it (its absence means it never
-- existed on a clean volume).
--
-- TODO: needs_prod_decisions pg_hba cert-map for multi-workload roles
-- (today every new workload that needs postgres adds a CREATE ROLE line
-- here AND a pg_hba.conf line).

CREATE ROLE server LOGIN;
GRANT ALL PRIVILEGES ON DATABASE app TO server;
GRANT ALL PRIVILEGES ON SCHEMA public TO server;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO server;

-- postgres-exporter needs read access to pg_stat_* views + the extension
-- objects it queries. The community exporter's queries also touch
-- pg_stat_database and pg_stat_user_tables; granting pg_monitor covers
-- the common surface without making the exporter a superuser.
CREATE ROLE "postgres-exporter" LOGIN;
GRANT pg_monitor TO "postgres-exporter";
