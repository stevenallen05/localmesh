# Agent

Heartbeat-only build (v0). Proves the container, config loader, slog wiring, and shutdown loop work end-to-end before collectors and gRPC land.

## Run

```bash
docker compose up agent
```

## Configure

- Defaults live in [`agent.toml`](./agent.toml).
- Override any value with `AGENT_*` env vars. See the comments in `agent.toml` for the full mapping table.

## What it does

- Loads config via Koanf (defaults < `agent.toml` < env).
- Starts slog with the configured level and format.
- Logs one `heartbeat` line every 5 seconds, tagged with `agent_id`.
- Exits cleanly on SIGTERM/SIGINT.

## What it does NOT do yet

See [`docs/DESIGN_DECISIONS.md`](../docs/DESIGN_DECISIONS.md) and the [v1 spec](../docs/superpowers/specs/2026-05-12-metrics-system-design.md) for the full scope. v0 has no collectors, no gRPC, no buffer, no server contact.
