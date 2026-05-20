# envoy_wip notes

In-repo TODOs about the templates and their CLI contract. These belong
next to the templates while the CLI is being written in parallel; they
will move into `docs/TODO.md` once the contract pins.

## CLI contract expectations

Each template's leading `{{/* ... */}}` block documents what context dict
it expects. The contract is not pinned. When the CLI lands its catalog
pass, the field names here may shift.

- **`envoy-mesh.compose.yaml.gotmpl`** consumes a project-wide context
  plus `.workloads` (list of per-app entries).
- **`workload-sidecar.envoy.yaml.gotmpl`** is rendered once per workload;
  context is one entry from `.workloads`.
- **`ingress-gateway.envoy.yaml.gotmpl`** consumes project-wide context
  plus `.routes` (list of `expose_via_ingress=true` workloads).
- **`egress-gateway.envoy.yaml.gotmpl`** consumes project-wide context
  plus `.externals` (declared external destinations across all workloads).
- **`_helpers.tmpl`** defines named blocks consumed via `{{ include }}`.

## Rendering pipeline

The CLI is expected to:

1. Read all `*.compose.yaml.gotmpl` across plugins, render each with the
   project-wide context, assemble into the final compose YAML.
2. Read each envoy template under `envoy_wip/templates/`, render per-role
   (and per-workload for the sidecar template), drop the outputs into
   `envoy_wip/rendered/` (gitignored). Each container mounts its
   rendered config at `/etc/envoy/envoy.yaml`.
3. Trigger step-ca to mint certs + emit `id.policy.json` per workload
   into `.secrets/policy/<workload>.policy.json`.

## Known gaps

- **Compose interpolation vs Sprig.** The compose template uses `{{ }}`
  Sprig markers; existing compose files use `${...}` env-interpolation.
  The CLI is expected to render Sprig first, then write a file that
  compose interpolates at `docker compose up` time.
- **Policy-loader.** Envoy doesn't parse the JSON policy file itself.
  An init step (compose-level init container or a shell script the
  sidecar wraps) must turn `id.policy.json` into the RBAC fragment that
  the sidecar's envoy config includes. Today the RBAC `policies` map
  is empty in every template.
  `TODO: needs_prod_decisions cert-policy loader for inbound + egress RBAC`
- **oauth2_filter secrets.** The ingress template references
  `/run/ingress-mesh/oauth2.secret.yaml` and `hmac.secret.yaml`. These
  are SDS-shaped files the CLI needs to mint (today they don't exist).
  Workaround for MVP is to skip the OIDC dance entirely and run with
  jwt_authn against a fixed test token until the secret pipeline lands.
- **Compose `network_mode: "service:..."` and katenary.** Same-pod
  sidecars don't have a clean katenary rendering yet (already filed in
  `docs/TODO.md`: katenary same-pod label for sidecars).
- **Egress catch-all wiring.** The workload-sidecar's outbound listener
  binds 127.0.0.1:15001 but the app dials its upstreams directly by
  hostname (e.g. `server:50051`). Real iptables redirection (Istio-style)
  is out of scope for compose; an interim is hostname overrides via
  `extra_hosts` or per-cluster named upstreams that bypass the catch-all.
  `TODO: needs_prod_decisions sidecar transparent egress in compose`

## Out of scope for this scaffold

- Linter (aspirational; documented rules only).
- step-ca template extension to emit `id.policy.json` (the file is
  hand-crafted in `policy/schema.example.json` for now).
- Replacing the existing ghostunnel sidecars in `docker-compose.yml`
  (envoy_wip lives in parallel; ghostunnel stays running until the
  swap is ready).
- Per-callee outlier detection (row 3, dropped to `needs_prod`).
- Rows 8, 9, 10, 11, 12, 15 (deferred; see `scratch.envoy-omakase.md`).
