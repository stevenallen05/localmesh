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
  hand-added to cadvisor / node-exporter compose today (envoy roles
  already carry them, emitted by the mesh plugin's template). A future
  localmesh CLI pass will gain `[[exports.metrics]]` in `plugin.toml`;
  localmesh will write the labels into `localmesh.compose.yaml`'s
  overlay block. Hand-added labels migrate to declarations at that
  point. `TODO: needs_prod_decisions localmesh emits prometheus.io/*
  labels`.

## LocalMesh follow-ups

Items deferred from the service-mesh and envoy-migration deliveries —
all flagged inline as `TODO: needs_prod_decisions <≤10 words>` at the
relevant call site so they're greppable. Listed here for visibility;
resolution is context-dependent.

- **PII enforcement, downstream layer.** No `setAttributes('enduser.*')`
  call sites exist downstream of the ingress envoy. Per-data-class regex
  scrubbing at Vector / Tempo / Loki is still SRE's choice when real PII
  contracts settle.
- **Default-deny mesh policy.** No `AuthorizationPolicy` enforcement in
  dev. Foundation-tier RBAC is allow-all-internal. The `mesh.exempt:
  "true"` compose label is the CEL predicate the policy generator will
  read in prod to write carve-outs. Picking the prod mesh runtime
  (Istio / Linkerd / Cilium) is the upstream decision; default-deny
  lands when that lands.
- **Per-caller RBAC via cert-as-policy (row 14).** Workload's cert
  encodes its allowed destinations; sidecar reads cert and writes the
  RBAC config the listener consumes. JSON sidecar file
  (`id.policy.json`) is the dev shape; prod migration swaps the loader,
  not the policy semantics. `TODO: needs_prod_decisions per-caller RBAC
  via cert-as-policy`.
- **Cert lifespan + SVID rotation.** Dev leaf certs have 7-day
  lifetimes; prod uses short-lived SPIRE-issued SVIDs (~1h) with
  automatic rotation. Whichever cert-delivery mechanism prod picks
  (cert-manager vs SPIRE Workload API vs Vault PKI) sets the rotation
  cadence.
- **CA key handling.** Dev's `localmesh/secrets/root_ca/rootCA-key.pem`
  lives on the host filesystem. Prod never has CA key in any workload
  pod; it's locked in the chosen issuer (SPIRE / Vault / cert-manager
  backend).
- **Strict EKU per role.** `plugin.toml`'s `[[services]]` entries don't
  carry `client`/`server` bools today — every cert is bidirectional. An
  EKU-per-role minter is straightforward but depends on whether prod's
  issuer respects the same axis.
- **IP SAN list tightening.** Dev IP SAN list is over-permissive (RFC1918
  / docker-bridge / loopback-IPv6 / `0.0.0.0`). Prod uses DNS-based
  identity exclusively; cert-manager SVIDs carry no IP SANs.
- **`/etc/hosts` seeding.** `make check-hosts` enforces required entries
  but doesn't write them. Real DNS in prod; a privileged-init script /
  mise hook / devcontainer feature for dev is the in-between.
- **Ingress wildcard cert.** `*.${PROJECT_NAME}.${LOCAL_DOMAIN}` on the
  ingress envoy cert. Prod uses per-host certs via cert-manager
  IngressRoute.
- **postgres-exporter strict perm wrapper.** Both postgres and
  postgres-exporter wrap their image entrypoints to copy bind-mounted
  certs into a postgres/nobody-owned location with `0600` .key.
  Goes away when cert delivery is sidecar / SVID-based.
- **`plugin.toml` schema growth.** Today's schema is `[identity]` +
  `[[services]]` (with `scheme` + `requires_auth`). Future sections
  (`[[externals]]` for declared egress allowlist, `[allows]` for
  default-deny, compliance flags per `project.toml` shape, katenary
  chart selection) land as the upstream features land.
- **plugin.toml → compose interpolation.** `.env`'s managed section is
  the dev bridge today. Replace with a build-step compose overlay
  generator (or a real compose extension if compose grows one).
- **multi-workload postgres roles.** `pg_hba.conf` binds each cert CN
  to one role today. `pg_ident.conf` cert-map for multi-workload
  aliasing is `TODO: needs_prod_decisions pg_hba cert-map for
  multi-workload roles`.
- **gRPC client-side instrumentation in Node.** The www → server gRPC
  call still lacks `@opentelemetry/instrumentation-grpc`. Pre-existing
  gap; carries over from the logging delivery.
- **Dex JWKS key rotation strategy.** Dev uses long-lived in-memory
  keys. Server's JWKS cache TTL is 15 min (configurable). Prod IdP swap
  rotates keys on its own schedule; TTL should follow.
  `TODO: needs_prod_decisions JWKS cache TTL for prod IdP rotation`.
- **Prod IdP swap.** Dex with built-in local connector + staticPasswords
  is dev-only. Prod swaps the whole `auth/` plugin's `dex` container
  for a real IdP (Okta / Keycloak / Auth0); the envoy `oauth2` filter +
  `jwt_authn` wiring carries over unchanged.
  `TODO: needs_prod_decisions IdP choice + SSO migration`.
- **Ingress `depends_on` dex healthy at boot.** Today the ingress envoy
  holds on `depends_on: dex condition: service_healthy`. Production "no
  auth = no traffic" needs a richer readiness/circuit shape; revisit
  when dex's healthcheck contract firms. `TODO: needs_prod_decisions
  ingress depends_on dex healthy at boot`.
- **Sidecar lifecycle binding to workload.** Compose `depends_on` orders
  startup but doesn't bind sidecar lifetime to workload lifetime. Prod
  uses pod-readiness probes; dev compose papers over it via `restart:
  unless-stopped`. `TODO: needs_prod_decisions sidecar lifecycle binding
  to workload`.
- **iptables init container vs eBPF (Cilium-style) for prod.** The
  per-workload init container does `apk add iptables && /init.sh` under
  `cap_add: ["NET_ADMIN"]`. Pattern matches Istio/Linkerd today; eBPF
  lands when the prod runtime picks it. `TODO: needs_prod_decisions
  iptables init container vs eBPF (Cilium-style) for prod`.
- **Multi-arch iptables-init image.** alpine:3.20 + apk covers
  linux-amd64. arm64 needs verification. `TODO: needs_prod_decisions
  multi-arch iptables-init image`.
- **Envoy admin locked down in prod.** Admin listener on `:15000` is
  bound openly in dev for DX. SRE policy in prod. `TODO:
  needs_prod_decisions envoy admin locked down in prod`.
- **Grafana auth via envoy in prod.** Dashboards are browsed openly in
  dev (`requires_auth = false` on observability/'s grafana service).
  Prod requires JWT gate or per-team SSO. `TODO: needs_prod_decisions
  grafana auth via envoy in prod`.
- **Declared egress allowlist via plugin.toml `[[externals]]` (row
  13).** Uniform allowlist v0 is the placeholder. Apps declare what
  they reach outside the mesh in `plugin.toml`; the egress envoy reads
  the declarations to build the cluster + origination map. `TODO:
  needs_prod_decisions declared egress allowlist via plugin.toml
  [[externals]]`.
- **Outlier detection (row 3).** Three-strikes ejection per cluster
  with default thresholds. Per-callee tuning needs the CLI to join
  caller↔callee at template time so the callee's labels flow into the
  caller's outlier config. `TODO: needs_prod_decisions per-callee
  outlier detection needs CLI caller-callee join`.
- **Retry / timeout schemas (rows 8–9).** Default 30s timeout, no
  retries on non-idempotent routes. Per-route override via `plugin.toml`
  schema additions when needed. `TODO: needs_prod_decisions retry /
  timeout schemas`.
- **Scope linter (row 10).** `plugin.toml [[services]].scope = internal
  | external` + linter requires declaration. The bind address branches
  follow. `TODO: needs_prod_decisions scope linter`.
- **Header stripping on egress (row 11).** `authorization`, `cookie`,
  `x-user-*`, `x-internal-*` stripped on egress by default; per-dest
  override map. `TODO: needs_prod_decisions header stripping on
  egress`.
- **Claim-aware downstream (row 12).** JWT claim → header at ingress
  only; downstream sidecars strip externally-sourced `x-user-*` on
  outbound so the ingress is the only entry point. `TODO:
  needs_prod_decisions claim-aware downstream`.
- **Claim-filter at workload (row 15).** Workload's cert encodes which
  user claims it may see; sidecar strips the rest. Pairs with row 14.
  `TODO: needs_prod_decisions claim-filter at workload`.
- **`MESH_*_{HOST,PORT,URL}` auto-emission.** CLI emits connection-
  string env vars per consumer's resolved `depends_on`. Defer until the
  app-side adoption pattern settles. `TODO: needs_prod_decisions
  MESH_*_{HOST,PORT,URL} auto-emission`.
- **Formalize app-loopback port discovery.** Apps bind plaintext on
  loopback (e.g. server :50052, www :3444). Today the ports are
  convention; a `plugin.toml [[services]].app_port` declaration plus a
  linter would harden the contract. `TODO: needs_prod_decisions
  app-loopback port discovery (convention vs declaration)`.
- **katenary same-pod sidecar.** No native compose construct expresses
  "second container in the same pod." Hand-patch the chart for now;
  upstream feature request candidate. `TODO: needs_prod_decisions
  katenary same-pod label for sidecars`.
- **postgres sidecar.** Postgres protocol's STARTTLS negotiation isn't
  transparent-proxyable through a generic TLS terminator. Postgres
  stays mesh-exempt with native mTLS. `TODO: needs_prod_decisions
  postgres sidecar requires protocol-aware proxying`.

## LocalMesh CLI follow-ups

Items deferred from the localmesh Go CLI delivery and the envoy
migration. All flagged inline as `TODO: needs_prod_decisions <≤10
words>` at the relevant call site so they're greppable.

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
  `localmesh/` once `docker compose --env-file` / config-loading tools
  improve enough that the relocation doesn't break the
  no-flag-needed path. `TODO: needs_prod_decisions .env move under
  localmesh once config-loading tools improve`.
- **mesh-exempt enforcement in `mtls mint`.** The Go CLI mints leaves
  for every `[[services]]` entry uniformly. The mesh-exempt label
  carve-out (skip cert minting for observability / logging / dex /
  postgres containers) is not yet ported into `mtls mint`. Cert minting
  for exempt containers is wasted work today but harmless. The
  `internal/mesh/edges` derivation already reads `mesh.exempt: "true"`;
  share that with the cert path. `TODO: needs_prod_decisions
  mesh.exempt detection from compose labels`.
- **Long-form volume rewrite in render.** `internal/render/rewrite.go`
  rewrites relative paths for `build.context`, short-form volumes,
  `configs.file`, `secrets.file`. Long-form `volumes:` with `source:`
  + `target:` is not yet rewritten — no plugin uses it. Add when a
  plugin adopts the long form. `TODO: needs_prod_decisions long-form
  volume rewriting when a plugin adopts it`.

## Dashboards

The default-dashboard set ships three files:

- `localmesh/service_catalog/database/postgres.json` — postgres health, query rates, pg_stat_statements
- `localmesh/service_catalog/observability/apm.json` — RED panels, traces waterfall, RPC server panels
- `localmesh/service_catalog/observability/cluster-health.json` — cadvisor container resource panels

A mesh-ingress dashboard against envoy's native `:15090/stats/prometheus`
surface + JSON access logs is `TODO: needs_prod_decisions ingress
dashboard rewrite against envoy metrics`. APM already covers cross-
service RPC RED via `tracing.http`, so the standalone ingress dashboard
isn't on the critical path.

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
  `localmesh/service_catalog/observability/docker-compose.yml`. It promotes
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

- **App-tier cert as the access-policy carrier.** mTLS certs for the
  `app` tier carry an "allowed to access" payload in x509 metadata.
  The axes follow whatever the org cares about. Common axes are
  destination URIs, OIDC claim names, data classes, billing buckets.
  Sidecar reads the cert at startup. Cert rotation rotates policy.
  Schema lives in step-ca's template, consuming code in the sidecar
  bootstrap. Both are aspirational beyond the simplest URI-allowlist
  axis. Envoy doesn't parse custom x509 OIDs natively. The MVP
  delivery shape is a JSON-sidecar file (`id.policy.json`) that step-ca
  emits alongside the cert. The envoy sidecar's init parses the file
  once and writes RBAC config the listener consumes. Cert rotation
  rotates both files together. Prod migration swaps the loader, not
  the policy semantics — the RBAC contract downstream stays identical.

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
