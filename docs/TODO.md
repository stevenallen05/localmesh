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
  `owned_by`); `scripts/secrets-gen.py` writes them into `.env`'s
  managed section as UPCASE_SNAKECASE for compose interpolation. The
  working bridge is the `.env` write; the proper replacement (compose
  overlay generator at build time, or a real compose extension once
  compose grows one) is `TODO: needs_prod_decisions plugin.toml →
  compose interpolation`. Tracked in DESIGN_DECISIONS Open.
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

## LocalMesh follow-ups

Items deferred from the 2026-05-18 service-mesh delivery — all flagged
inline as `TODO: needs_prod_decisions <≤10 words>` at the relevant call
site so they're greppable. Listed here for visibility; resolution is
context-dependent.

- **PII enforcement.** The PII-at-ingress rule (`enduser.*` stamps only at
  www's API-route entry) is documented + code-reviewed today. Mechanical
  enforcement (Vector / Tempo regex scrubbing, lint rule on `.setAttributes`
  call sites, ESLint custom rule, semconv attribute allowlist on
  downstream services) is deferred. Picking the right enforcement layer
  depends on real-world PII contracts SRE settles on.
- **Default-deny mesh policy.** No `AuthorizationPolicy` enforcement in
  dev. The `mesh.exempt: "true"` compose label is the CEL predicate the
  policy generator will read in prod to write carve-outs. Picking the
  prod mesh runtime (Istio / Linkerd / Cilium) is the upstream
  decision; default-deny lands when that lands.
- **Cert lifespan + SVID rotation.** Dev certs have 10-year lifetimes;
  prod uses short-lived SPIRE-issued SVIDs (~1h) with automatic
  rotation. Whichever cert-delivery mechanism prod picks (cert-manager
  vs SPIRE Workload API vs Vault PKI) sets the rotation cadence.
- **CA key handling.** Dev's `.secrets/certs/ca.key` lives on the host
  filesystem. Prod never has CA key in any workload pod; it's locked in
  the chosen issuer (SPIRE / Vault / cert-manager backend).
- **Strict EKU per role.** `plugin.toml`'s `[[certs]]` entries don't carry
  `client`/`server` bools today — every cert is bidirectional. A
  step-cli template enforcing per-role EKU is straightforward but
  depends on whether prod's issuer respects the same axis.
- **IP SAN list tightening.** Dev IP SAN list is over-permissive (RFC1918
  / docker-bridge / loopback-IPv6 / `0.0.0.0`). Prod uses DNS-based
  identity exclusively; cert-manager SVIDs carry no IP SANs.
- **`/etc/hosts` seeding.** `make check-hosts` enforces required entries
  but doesn't write them. Real DNS in prod; a privileged-init script /
  mise hook / devcontainer feature for dev is the in-between.
- **`tools/step` vendoring.** Single binary committed to the repo today.
  Replace with bootstrap script (mise / nix / asdf) or a fetch-by-
  checksum step in `make setup`.
- **Caddy wildcard cert.** `*.${PROJECT_NAME}.${LOCAL_DOMAIN}` on the
  ingress cert. Prod uses per-host certs via cert-manager IngressRoute.
- **postgres-exporter strict perm wrapper.** Both postgres and
  postgres-exporter wrap their image entrypoints to copy bind-mounted
  certs into a postgres/nobody-owned location with `0600` .key.
  Goes away when cert delivery is sidecar / SVID-based.
- **`plugin.toml` schema growth.** Today's schema is `[identity]` +
  `[[services]]`. Future sections (`[allows]` for default-deny, compliance
  flags per `project.toml` shape, katenary chart selection) land as
  the upstream features land.
- **plugin.toml → compose interpolation.** `.env`'s managed section is
  the dev bridge today. Replace with a build-step compose overlay
  generator (or a real compose extension if compose grows one).
- **multi-workload postgres roles.** `pg_hba.conf` binds each cert CN to
  one role today. `pg_ident.conf` cert-map for multi-workload aliasing
  is `TODO: needs_prod_decisions pg_hba cert-map for multi-workload roles`.
- **gRPC client-side instrumentation in Node.** The www → server gRPC
  call still lacks `@opentelemetry/instrumentation-grpc`. Pre-existing
  gap; carries over from the logging delivery.

## Dashboards

The default-dashboard set ships four files:

- `service_catalog/caddy/ingress.json` — ingress request rates, response codes, upstream health
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
