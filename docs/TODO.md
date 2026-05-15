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
- **Swap dashboard `min_level` line-filter for a real Loki label
  filter.** Once every conforming source emits a real `level`, the
  dashboard can switch from `|~ "$min_level"` to `| level=~"..."` —
  exact, cheaper, no regex false positives.
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

## Multi-environment portability

- **k8s overlay for filelog `include:` path.** Compose tails
  `/var/lib/docker/containers/*/*-json.log`; kubelet writes to
  `/var/log/containers/*.log`. The k8s helm chart needs an override that
  swaps the path. Document the override pattern alongside the receiver
  config.
