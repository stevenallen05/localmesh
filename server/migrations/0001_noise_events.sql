-- Demo-only table. Each PrintPostgresStats RPC writes one row, which
-- makes pg_stat_database move and gives pg_tracing both an INSERT and a
-- SELECT to capture per click. No application data lives here; real
-- product use-cases will define their own schemas.
CREATE TABLE noise_events (
    id          BIGSERIAL    PRIMARY KEY,
    kind        TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
