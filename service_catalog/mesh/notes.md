# mesh plugin notes

In-repo TODOs about the templates and their CLI contract. These belong
next to the templates while the package is being iterated; they migrate
into `docs/TODO.md` once the shape stabilizes.

## CLI contract

Each template's leading `{{/* ... */}}` block documents the context it
expects. The localmesh CLI's `internal/mesh/` package owns the render
side:

- **`docker-compose.yaml.gotmpl`** consumes project-wide context plus
  `.Workloads` (list of `{Name}` derived from the catalog).
- **`templates/workload-sidecar.envoy.yaml.gotmpl`** is rendered once
  per workload; context is one entry from the edges set (workload name
  + the peer clusters keyed off destination port).
- **`templates/ingress-gateway.envoy.yaml.gotmpl`** consumes project-
  wide context plus `.Routes` (one entry per service with
  `expose_via_ingress = true`).
- **`templates/egress-gateway.envoy.yaml.gotmpl`** consumes project-
  wide context plus `.Externals` (declared external destinations —
  uniform allowlist v0 today; `TODO: needs_prod_decisions declared
  egress allowlist via plugin.toml [[externals]]`).
- **`templates/_helpers.tmpl`** defines named blocks consumed via
  `{{ include }}` (admin / tracing / access_log / mtls / stats).

## Rendering pipeline

`localmesh build` runs:

1. Read all `*.compose.yaml.gotmpl` across plugins, render each with
   the project-wide context, assemble into
   `.localmesh/localmesh.compose.yaml`.
2. Build the catalog (`{container → Service}`) from project + plugins.
3. Walk each workload's compose `depends_on`, intersect with the
   catalog, emit `MeshEdge` per workload. Fail the build on port
   collisions.
4. Render each envoy template under `service_catalog/mesh/templates/`
   per-role (and per-workload for the sidecar template), drop outputs
   into `.localmesh/envoy/` (gitignored). Each container mounts its
   rendered config at `/etc/envoy/envoy.yaml`.
5. Emit per-workload iptables-init scripts into
   `.localmesh/envoy/<container>-mesh-init.sh`, mounted onto each
   init container.

Certs are minted by `mtls mint` (native crypto/x509, SPIFFE URI SAN,
7-day lifetime) into `.localmesh/secrets/<container>/`. Delivery into
envoy containers happens via docker compose `secrets:` (avoids the
envoy-image uid 101 vs host uid 1000 mismatch a bind-mount hits).

## Known gaps

- **Policy-loader.** Envoy doesn't parse the JSON policy file itself.
  An init step (compose-level init container or a shell script the
  sidecar wraps) must turn `policy/<workload>.policy.json` into the
  RBAC fragment that envoy includes. Today the RBAC `policies` map is
  empty in every template. `TODO: needs_prod_decisions per-caller RBAC
  via cert-as-policy (row 14)`.
- **Compose `network_mode: "service:..."` and katenary.** Same-pod
  sidecars don't have a clean katenary expression yet. `TODO:
  needs_prod_decisions katenary same-pod label for sidecars`.
- **Egress allowlist v0.** Uniform allow-all in dev. Declared per-
  destination shape lands when `plugin.toml [[externals]]` does.
  `TODO: needs_prod_decisions declared egress allowlist via plugin.toml
  [[externals]]`.
- **Multi-arch iptables-init image.** alpine:3.20 + `apk add iptables`
  covers linux-amd64. arm64 needs verification. `TODO:
  needs_prod_decisions multi-arch iptables-init image`.

## Out of scope for this plugin

- Linter (aspirational; documented rules only).
- Cert-mint extension to emit per-workload `id.policy.json` (the file
  is hand-crafted in `policy/schema.example.json` for now).
- Per-callee outlier detection — needs the CLI to join caller↔callee
  at template time. A uniform default applies today.
