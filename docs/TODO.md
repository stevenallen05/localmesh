# Known gaps — demo polish

Scoped to **this take-home project**. Anything that's a real production
hardening concern lives in `DESIGN_DECISIONS.md` (or doesn't exist yet) —
not here.

## Logging

- **`service_name=unknown_service` on every container log.** filelog tails
  raw docker JSON files and doesn't know the container's service identity.
  Two paths: have apps (server, www) export OTLP logs directly with
  `OTEL_RESOURCE_ATTRIBUTES` (clean, but doesn't help non-app containers),
  or enrich at the collector with `docker_observer` + an attributes
  processor that maps `container_id` → service name.
- **`module_name`/`owned_by` missing on container logs.** Same root cause,
  same fix paths.
- **`service-glance` dashboard's logs panel uses a regex body match**
  (`|~ "(?i)$service"`) instead of a proper label filter — works for the
  demo, brittle in practice. Resolves once the above gap is filled.

## Dashboards

- **`cluster-health` includes the host's `agentbox-*` containers.**
  cadvisor scrapes everything on the docker daemon, including the
  dev-environment container. A `name!~"agentbox.*"` filter on each panel
  would clean it up; doing it centrally needs a recording rule or
  collector-side relabel.
- **`infra-modules` service-graph table has no `module_name` column.**
  Tempo's `service_graphs` processor emits edges at the service level;
  there's no built-in way to project resource attrs onto them. Either
  derive module-of-service from a separate query, or build the same edge
  set off span pairs in a TraceQL query.
- **`service-glance` RED ↔ USE join is by container `name` regex.**
  `traces_spanmetrics_*` carries `service`, `container_*` metrics carry
  `name` — different labels. Today the dashboard hopes the compose name
  matches the service. A recording rule (or relabel) that adds
  `service` to container metrics would make the join exact.

## Postgres / Rust integration

- **What the server uses postgres for is undecided.** The compose wiring
  (`DATABASE_URL`, `depends_on: postgres healthy`) is in place; no Rust
  client yet. Options sketched in conversation: service registry /
  module-owner source-of-truth, audit log on Grafana mutations, plain
  events log. Pending decision.
- **Migration tooling not chosen.** When the server gains a postgres
  client, pick a migration runner (`sqlx::migrate!`, `refinery`, or
  external `dbmate` / `goose` container) — this is one of the things the
  dev-env should demonstrate alongside pg_tracing.

## OTel collector

- **OTTL `transform/database` logs an auto-correct warning at startup.**
  Statements like `set(attributes["x"], ...)` in `context: resource` get
  rewritten to `set(resource.attributes["x"], ...)` automatically.
  Functional but noisy — rewrite the statements to the prefixed form.
- **Deprecated component aliases.** Collector 0.152 wants `otlp_grpc`,
  `otlp_http`, `file_log` instead of `otlp`, `otlphttp`, `filelog`.
  Rename now, before the aliases get removed.

## Multi-environment portability

- **k8s overlay for filelog `include:` path.** Compose tails
  `/var/lib/docker/containers/*/*-json.log`; kubelet writes to
  `/var/log/containers/*.log`. The k8s helm chart needs an override that
  swaps the path. Document the override pattern alongside the receiver
  config.
