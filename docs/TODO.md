# Known gaps — demo polish

Scoped to **this take-home project**. Anything that's a real production
hardening concern lives in `DESIGN_DECISIONS.md` (or doesn't exist yet) —
not here.

## My thoughts

First, make a .env to mock the project.toml (will eventually be .env.sample,
just leave a todo note at the top of .env). Include int/ext domains, project name, etc -just what's
needed to provide a bare minimum functional portable demo.

Second, the URIs, namespaces, tags, etc used across the compose & catalog need to 
be double-checked. Assume the env vars are long-term sufficient, and populating them
in "better" ways is an exercise to be done later. Then interpolate with compose 
best-practices

Third, the collectors need to be limited to the sensible default boundary. 
Collectors should be restricted to just devices on the same compose network.
The only prod note that's needed is that the metrics collectors are
dev-only; the same data is available on prod, but those collectors are owned
by ops. The dev env version is just to provide fidelity and give the same 
observability DX between prod and 'my machine'

## Cross-cutting conventions

- **`metrics.*` compose-label namespace.** `metrics.service_name` is the
  first inhabitant. Future infra-facing labels (trace sampling
  overrides, log-routing hints, etc.) should follow the same
  `<subsystem>.<purpose>` shape; document the rule alongside the
  katenary label conventions in
  [`katenary-top-seven.md`](../engineering/rules/katenary-top-seven.md)
  so additions don't drift.

## Logging follow-ups

- **Per-source log-level extractors for non-app containers.** Postgres,
  otel-collector, grafana, tempo, loki, vm, cadvisor, node-exporter
  each speak a different format. Either `docker_observer` enrichment by
  container id or per-source filelog operators. Until then,
  `service-glance`'s default `INFO+` filter hides these sources.
- **Validate `level` enum at the collector.** Drop / coerce malformed
  `level` values from non-conforming sources before they hit Loki, to
  bound the label's cardinality in prod.
- **Migrate Grafana Loki's `trace_id` derived field from body-regex to
  structured-metadata reference.** The matcher works today via
  `matcherRegex` on the body; field-based is cleaner and doesn't break
  if body format changes.

## OTel collector

- **OTTL `transform/database` logs an auto-correct warning at startup.**
  Statements like `set(attributes["x"], ...)` in `context: resource` get
  rewritten to `set(resource.attributes["x"], ...)` automatically.
  Functional but noisy — rewrite the statements to the prefixed form.
- **Deprecated component aliases.** Collector 0.152 wants `otlp_grpc`,
  `otlp_http`, `file_log` instead of `otlp`, `otlphttp`, `filelog`.
  Rename now, before the aliases get removed.

## Multi-user identity scaffolding

End-to-end identity flow from the www edge through gRPC to the
server. Kept minimal — no JWT minted or validated, no roles, no
per-user data scoping. Production story (service mesh + SPIFFE/SPIRE
+ pass-through user JWT) is summarised in [`AUTH.md`](./AUTH.md); the
implementation plan is in
[`superpowers/specs/2026-05-15-identity-propagation-design.md`](./superpowers/specs/2026-05-15-identity-propagation-design.md).

- **One hardcoded dev user.** `DEV_USER_ID` / `DEV_USER_EMAIL` /
  `DEV_USER_NAME` in `.env`; a `getUser(req)` helper reads
  `X-Forwarded-*` headers first and falls back to the env. A
  forward-auth proxy would populate those headers in prod.
- **Identity on the wire.** Three gRPC metadata keys (`x-user-id`,
  `x-user-email`, `x-user-name`) attached at the www call site.
- **Server-side reception.** Tonic interceptor extracts the keys
  into a `User` struct and inserts it into request extensions;
  each handler reads the extension and stamps `user.id` /
  `user.email` on its OTel span. Identity is optional — missing
  keys produce no extension and empty-string attrs, no error.
- **Demo evidence.** All three existing demo buttons (PrintPostgresStats,
  ListGrafanaDatasources, TestRPC) carry identity through; toggling
  `DEV_USER_*` and restarting demonstrates the plumbing is
  identity-agnostic.
- **DESIGN_DECISIONS row** (*Identity propagation*) lands with the
  implementation.

## Dashboards

- **Outbound RPC client metrics (`rpc.client.duration`).** The imported
  APM dashboard's "RPC outbound" panels query `rpc_client_duration_*`,
  which only the *caller* emits. We instrument the Rust gRPC server via
  a Tower layer (`rpc.server.duration`); the www→server gRPC call from
  Node has no client-side gRPC instrumentation wired up. Add
  `@opentelemetry/instrumentation-grpc` to `www/instrumentation.ts`'s
  `instrumentations: [...]` so those panels populate. Server-side panels
  work — this is a one-direction gap.

- **`rpc.server.duration` is recorded in milliseconds, not seconds.**
  Stable OTel semconv says seconds, but in `opentelemetry-rust` 0.31 both
  `HistogramBuilder::with_boundaries()` and `MeterProviderBuilder::with_view()`
  are silently ignored — the exported histogram always uses the SDK's
  default boundaries `[0, 5, 10, 25, …, 10000]`, which are sized for ms.
  Recording in seconds makes every realistic latency fall into the
  `[0, 5]` bucket and clamps `histogram_quantile` to 5. Recording in ms
  fits the default buckets and gives usable percentiles. The dashboard's
  RPC server panels are also milliseconds-shaped (the upstream community
  dashboard was written for ms). Revisit when the SDK honors custom
  boundaries — at that point switch to seconds + explicit buckets and
  flip the dashboard's RPC queries back to `_seconds_`.

## Multi-environment portability

- **k8s overlay for filelog `include:` path.** Compose tails
  `/var/lib/docker/containers/*/*-json.log`; kubelet writes to
  `/var/log/containers/*.log`. The k8s helm chart needs an override that
  swaps the path. Document the override pattern alongside the receiver
  config.
