-- Runs once on first cluster init (postgres image convention:
-- /docker-entrypoint-initdb.d/*.sql executes against ${POSTGRES_DB}).
-- pg_tracing is loaded via shared_preload_libraries (see compose command),
-- so the extension is available before this script runs.

CREATE EXTENSION IF NOT EXISTS pg_tracing;

-- Matches server/migrations/0002_hello_messages.sql. The db-probe consumer
-- INSERTs into this table; server's sqlx::migrate!() is idempotent against
-- it when server is re-wired.
CREATE TABLE IF NOT EXISTS hello_messages (
    id          BIGSERIAL    PRIMARY KEY,
    message     TEXT         NOT NULL,
    jwt_issuer  TEXT         NOT NULL,
    jwt_subject TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
