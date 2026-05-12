# Server

`Greeter.SayHello` only (v0). Proves the tonic server, proto codegen wiring,
and NextJS → Rust gRPC path before `MetricsIngest` / `MetricsQuery` land in
their own spec.

## Run

```bash
docker compose up server
```

## Configure

- `SERVER_ADDR` — bind address. Default `0.0.0.0:50051`.
  Single knob today; routes through a Koanf-equivalent loader once a second
  knob shows up.

## What it does

- Listens on the configured TCP address.
- Implements `hello.Greeter/SayHello`. Empty `name` → `hello, world`.

## What it does NOT do yet

No TimescaleDB connection, no auth, no `MetricsIngest`, no `MetricsQuery`.
See [`docs/DESIGN_DECISIONS.md`](../docs/DESIGN_DECISIONS.md) and the
[v1 spec](../docs/superpowers/specs/2026-05-12-metrics-system-design.md).

## Proto codegen

`build.rs` writes `src/proto/hello.rs` from `../proto/hello.proto` on every
`cargo build` and commits the output. Lets reviewers see the wire shape in
diffs without running `cargo` first, and keeps the Docker build hermetic.
