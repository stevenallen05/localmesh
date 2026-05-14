-- Runs once on first cluster init (postgres image convention:
-- /docker-entrypoint-initdb.d/*.sql executes against the default db).
-- pg_tracing is loaded via shared_preload_libraries (see compose command:),
-- so the extension is available before this script runs.

CREATE EXTENSION IF NOT EXISTS pg_tracing;

-- Demo table the Rust server's Persist RPC writes into. Schema is intentionally
-- thin — the point is generating SQL traffic that pg_tracing emits spans for.
CREATE TABLE IF NOT EXISTS events (
    id          BIGSERIAL PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload     TEXT NOT NULL
);
