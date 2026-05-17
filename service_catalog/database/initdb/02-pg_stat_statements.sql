-- Runs once on first cluster init. pg_stat_statements is loaded via
-- shared_preload_libraries in the postgres command (see compose), so the
-- extension is available before this script runs.
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
