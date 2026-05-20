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

Fourth, revisit cadvisor's project-filter regex (`'${PROJECT_NAME}|'` with
the empty alternative) introduced when prometheus moved to docker_sd
discovery. The empty branch is load-bearing: it lets non-cadvisor series
through the shared `discovered` job because they don't carry
`container_label_com_docker_compose_project`. Worth re-reading once muscle
memory on Prometheus relabel semantics is back — there might be a cleaner
way to scope the filter to cadvisor's series only (e.g., job split, or
matching `__name__=~"container_.+"` instead of the empty-string trick).

## Cross-cutting conventions

- **`metrics.*` compose-label namespace.** Inhabitants are
  `metrics.service_name` (cadvisor metric label + Vector log enrichment),
  `metrics.module_name` and `metrics.owned_by` (Vector log enrichment).
  Future infra-facing labels (trace sampling overrides, log-routing
  hints, etc.) should follow the same `<subsystem>.<purpose>` shape;
  document the rule alongside the katenary label conventions in
  [`katenary-top-seven.md`](../engineering/rules/katenary-top-seven.md)
  so additions don't drift.
- **Identity source consolidation.** `plugin.toml` is now the source of
  truth for the per-plugin halves of the identity tuple (`module_name`,
  `owned_by`); `localmesh build` (`internal/envwriter`) writes them
  into `.env`'s managed section as UPCASE_SNAKECASE for compose
  interpolation. The working bridge is the `.env` write; the proper
  replacement (compose overlay generator at build time, or a real
  compose extension once compose grows one) is `TODO:
  needs_prod_decisions plugin.toml → compose interpolation`. Tracked in
  DESIGN_DECISIONS Open.
- **Plugin-conventions linter.** Identity tuple binding + `README.md`
  template compliance ship as documentation today. Land a v0+1 linter
  that verifies every plugin's identity labels match its directory slug,
  every app-tier service uses `module_name=app`, and every plugin with a
  consumer surface ships an 8-heading `README.md`. Tracked in
  DESIGN_DECISIONS Open.

## Logging follow-ups

- **Migrate Grafana Loki's `trace_id` derived field from body-regex to
  structured-metadata reference.** Body-regex still works post-Vector
  (Vector ships `trace_id` in the JSON body for app events, and as
  structured metadata via the loki sink). The cleaner field-based pivot
  in Grafana 11 / Loki 3.x needs verification — `matcherType: label`
  matches stream labels, not structured metadata; the path is probably
  an `internalLink` with raw LogQL on the linked side.

## OTel collector

- **OTTL `transform/database` logs an auto-correct warning at startup.**
  Statements like `set(attributes["x"], ...)` in `context: resource` get
  rewritten to `set(resource.attributes["x"], ...)` automatically.
  Functional but noisy — rewrite the statements to the prefixed form.
- **Deprecated component aliases.** Collector 0.152 wants `otlp_grpc`,
  `otlp_http`, `file_log` instead of `otlp`, `otlphttp`, `filelog`.
  Rename now, before the aliases get removed.
- **docker socket scope.** otel-collector mounts
  `/var/run/docker.sock:ro` for `docker_sd_configs` discovery. It also
  runs as `user: "0:0"` for socket-group access. cadvisor sets the
  precedent for privileged-socket access. Prod uses
  `kubernetes_sd_configs` reading pod labels — no socket. `TODO:
  needs_prod_decisions docker socket scope`.
- **localmesh integration for `prometheus.io/*` labels.** Labels are
  hand-added to cadvisor / node-exporter compose today. A future
  localmesh CLI pass will gain `[[exports.metrics]]` in `plugin.toml`;
  localmesh will write the labels into `localmesh.compose.yaml`'s
  overlay block. Hand-added labels migrate to declarations at that
  point. `TODO: needs_prod_decisions localmesh emits prometheus.io/*
  labels`.

## LocalMesh CLI follow-ups

Items deferred from the localmesh Go CLI delivery. All flagged inline as
`TODO: needs_prod_decisions <≤10 words>` at the relevant call site so
they're greppable.

- **mkcert via go module.** mkcert v1.4.4 vendored as a binary at
  `localmesh_src/tools/mkcert` today (subprocess invocation from
  `internal/ca`). Importing mkcert as a Go module would remove the
  binary commit and align with the rest of the Go CLI's dep story.
  `TODO: needs_prod_decisions go mod import of mkcert`.
- **Multi-arch mkcert.** Linux-amd64 binary only. macOS-arm64 / Linux-
  arm64 contributors hit a clear "wrong arch" error today; a real
  arch-detect step (or the go-mod import above) closes the gap.
  `TODO: needs_prod_decisions detect arch beyond linux-amd64`.
- **`validate` verb.** Schema-lint of `project.toml` + `plugin.toml`
  deferred until a Go schema validator is picked. Five v0 verbs ship
  without it. `TODO: needs_prod_decisions validate verb when go schema
  validator picked`.
- **`install` verb (production).** Verb name reserved for a future
  production deployment command (chart push, registry auth, etc.).
  Distinct from `ca install` (host trust-store). Today the CLI exits
  with "unknown verb" — that's the intentional placeholder.
  `TODO: needs_prod_decisions install verb for production deployment`.
- **`.env` location.** Stays at repo root today because compose's
  default lookup expects it there. Aspirational move under
  `.localmesh/` once `docker compose --env-file` / config-loading tools
  improve enough that the relocation doesn't break the
  no-flag-needed path. `TODO: needs_prod_decisions .env move under
  .localmesh once config-loading tools improve`.
- **Long-form volume rewrite in render.** `internal/render/rewrite.go`
  rewrites relative paths for `build.context`, short-form volumes,
  `configs.file`, `secrets.file`. Long-form `volumes:` with `source:`
  + `target:` is not yet rewritten — no plugin uses it. Add when a
  plugin adopts the long form. `TODO: needs_prod_decisions long-form
  volume rewriting when a plugin adopts it`.

## Dashboards

The default-dashboard set ships three files:

- `service_catalog/database/postgres.json` — postgres health, query rates, pg_stat_statements
- `service_catalog/observability/apm.json` — RED panels, traces waterfall, RPC server panels
- `service_catalog/observability/cluster-health.json` — cadvisor container resource panels

### Identity-pivot reconciliation

**Outcomes.**

- **One identity-pivot vocabulary across every default dashboard.** Pickers
  expose `namespace` and `service` (or `container_name`, where cadvisor labels
  are the natural axis). No `environment` picker — dev only runs one
  environment and multi-env promotion has its own separate design. The picker
  shape mirrors the canonical identity tuple in
  [`plugin-conventions.md`](../engineering/rules/plugin-conventions.md) §1,
  so a dev who learns it on one dashboard reads every other dashboard the
  same way.
- **Every panel query respects the picker that drives it.** Identity-axis
  label selectors (`service_namespace=~"$namespace"`,
  `service_name=~"$service"`, or the cadvisor equivalent
  `service=~"$container_name"`) appear on per-service / per-container panels.
  Headline aggregate panels (cluster totals, "running containers", title
  banners) stay unfiltered. If a panel cannot accept the filter cleanly
  (mixed-source merges, metrics that don't carry the label), leave it alone
  — the picker is allowed to be a no-op for that panel rather than break it.
- **No stale `environment` references in non-picker sites.** Panel
  descriptions, dashboard header markdown (`<h1>` blocks), and Tempo
  trace-search `tags:` arrays sometimes carry
  `${deployment_environment_name}` or equivalent. Drop these in the same
  pass as the picker removal so the dashboard's UI text matches the
  picker's surface.

**Why.** A clean identity pivot across every default dashboard, aligned
with the now-canonical identity tuple from `plugin-conventions.md`.
Multi-env promotion reintroduces the environment dimension later; until
then it is dead UI that invites picker-blindness.

**Known constraints for the reconciliation.**

- **cadvisor metrics don't carry `service_namespace` natively** — they are
  scraped directly, not via the OTel-collector resource pipeline. A
  `namespace` picker on a cadvisor-shaped dashboard (cluster-health,
  ingress) will be informational unless joined with `target_info`.
  Pragmatic call is to filter cadvisor panels by `service`/`name` only and
  let the `namespace` picker stand as a cross-dashboard convention
  placeholder; document the asymmetry in the picker description.
- **The cadvisor `service` label is set by `metric_relabel_configs`** in
  `service_catalog/observability/docker-compose.yml`. It promotes
  `container_label_metrics_service_name` (and falls back to the container
  `name`) so cadvisor series can pivot on the same identity vocabulary as
  OTel-emitted series. The picker query is
  `label_values(container_last_seen, service)` (or whichever cadvisor
  metric is canonical at the time of reconciliation).
- **OTel-shaped identity labels are `service_namespace` / `service_name`**
  (underscored — Prometheus mangles the dots in OTel attribute names).
  Picker queries source from
  `label_values(target_info, service_namespace)` /
  `label_values(target_info, service_name)` when both vars are intended
  for OTel panels. Avoid `label_values(<label>)` bare-form on Victoria
  Metrics — it ignores the index hint that `target_info`-scoped form
  provides.
- **Mixed-target panels (`up` + `traces_spanmetrics_calls_total`)** will
  accept the namespace filter on the traces target but not on `up`
  (Prometheus self-scrape doesn't carry resource attrs). If such a panel
  breaks under a global filter, leave it unfiltered rather than splitting
  the targets.

### Content gaps (independent of the pivot work)

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

# Aspirational

Items not on the MVP critical path. Each articulates a desired prod
state where the dev shape is sketched or working, but the full
production form needs more design before it ships. The right shape
usually depends on which prod runtime, IdP, mesh, or CA backend the
org picks.

- **External-destination declaration.** Apps declare what they reach
  outside the mesh in `plugin.toml`. The take-home stops at the
  declaration. Prod enforcement is the aspirational half. Options are
  mechanical block at the egress gateway, SWG integration, allowlist
  ingestion. The right choice depends on the enforcement complexity
  the org needs. `TODO: needs_prod_decisions external-destination
  enforcement depth`.

- **East-west caller declaration.** Apps declare which other in-mesh
  services they're allowed to dial. The dev version extrapolates from
  which plugins the project manifest imports. mTLS client certs get
  minted with the explicit list of allowed SPIFFE URIs embedded in
  the x509. Elaborate enforcement (per-method allowlists,
  time-bounded grants) is aspirational.

- **Safety-net defaults.** Default timeouts, circuit breakers, and
  retry caps. Sensible defaults are the omakase move. They would
  surface as panel noise in MVP. Timed-out and ejected counters fire
  on every cold-start blip without context for what's normal. Land
  the defaults once a baseline is observable. Tune from there.
  `TODO: needs_prod_decisions safety-net defaults need a baseline`.

- **Convention + linter.** Conventions land in the take-home.
  `plugin.toml` schema additions, naming rules, claim-availability
  declarations. The build-time linter that verifies them is
  aspirational. Rules are documented; mechanical enforcement is not.

- **Mechanical in prod, working replica in dev.** The general posture
  for everything in this section. Anything flagged `TODO:
  needs_prod_decisions` or parked in DESIGN_DECISIONS Open lives in
  this same spirit. Local dev's job is the working replica, not the
  enforcer.
