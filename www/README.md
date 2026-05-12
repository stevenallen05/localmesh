# www

Next.js App Router frontend (v0). One Server Component calls the Rust gRPC
server and renders the reply. No styling, no client JS; proves the
SSR-gRPC path before the real UI lands.

## Run

```bash
docker compose up www
```

Then open <http://localhost:3000>.

## Configure

- `SERVER_ADDR` — Rust gRPC address. Default `127.0.0.1:50051`; compose sets
  `server:50051` to reach the service across the docker network.

## gRPC client

Server Components call `@grpc/grpc-js` directly with `@grpc/proto-loader`
reading `../proto/hello.proto` at runtime. No codegen step on the TS side —
proto-loader uses the `.proto` directly. The Rust side commits tonic-build
output for diff-visibility; see [`docs/DESIGN_DECISIONS.md`](../docs/DESIGN_DECISIONS.md).

TODO: prod swaps the direct client for a proper LB (envoy / grpc-web /
service mesh). See "gRPC LB" in DESIGN_DECISIONS.md.
