# Cleanup tracker

Follow-up work created or parked by individual design changes. Each entry names the change that introduced it, a one-line rationale, and where the work lives. This file is the live status. Specs may reference it from a `## CLEANUP inventory` section; the spec captures intent, this file captures status.

## Open

### 1. `[[services]].scheme` removal

Source: `docs/superpowers/specs/2026-05-22-plugin-config-inversion-design.md` §11.1 (parked from the precursor `2026-05-22-plugin-env-vars-design.md`).

Rationale: validation-only field. Its only consumer is a closed-set regex check. The wire protocol it documented is now named by `exports.template` (`redis://…`, `postgresql://…`). Carrying it forward duplicates the source of truth.

Touches:

- `localmesh_src/internal/manifest/manifest.go` — drop `Service.Scheme`, `validateServiceSchemes`, `validSchemes`.
- `localmesh_src/internal/manifest/manifest_test.go` — delete every test whose name contains `Scheme`.
- `project.toml` — drop `scheme = "…"` lines on every `[[services]]`.
- Every `localmesh/service_catalog/*/plugin.toml` — drop `scheme = "…"` lines.
- `localmesh_src/testdata/sample_catalog/**/plugin.toml` — drop `scheme = "…"` lines.

### 2. Per-plugin README Environment table

Source: config-inversion §11.3.

Rationale: once `exports` is in place the Environment table duplicates manifest data. Either auto-generate from exports or trim the section to a pointer at the consumer's resolved destination.

Touches:

- `docs/engineering/rules/plugin-conventions.md` §3 — Environment subsection rules.
- Every plugin's `README.md` — once §3 lands.

### 3. `plugin-conventions.md` §4 env-var naming refresh

Source: config-inversion §11.4.

Rationale: §4 documents `<PLUGIN>_*` and `<CONTAINER>_*` patterns for envwriter-emitted vars. Phase 1 adds `<PLUGIN>_<CONFIG>` for config_var-driven internal env vars and dev-chosen names for consumer-facing exports. The section needs the new rules once Phase 1 lands.

Touches: `docs/engineering/rules/plugin-conventions.md` §4.

### 4. `project.toml` ↔ `deploy.toml` sync

Source: config-inversion §11.5.

Rationale: a plugin's slug appears in both files (selection vs instantiation). Phase 1 keeps them in manual sync. Phase 3 (`localmesh add`) makes it automatic. Until then a manifest-loader cross-check could warn on "selected but never instantiated" (the reverse — instantiated but not selected — is already caught by `LoadDeploy` validation).

Touches: `localmesh_src/internal/manifest/manifest.go` cross-validation; eventually a `localmesh add` verb.

### 5. Identity tuple rebind (Phase 2 trigger)

Source: config-inversion §11.6 (and Phase 2 in §8).

Rationale: multi-instance support requires `service_name` to become the per-instance OTel `service.name` and per-instance container name; `module_name` stays the plugin-kind axis. The current convention (`service_name` = container slug, `module_name` = plugin slug, one-per-container-role) doesn't survive two instances of the same source.

Touches:

- `docs/engineering/rules/plugin-conventions.md` §2 — new binding.
- Every plugin's `docker-compose.yml` once it adopts per-instance rendering — moves from plain `.yml` to `.gotmpl` keyed by `service_name`.
- Per-instance cert minting — `localmesh_src/internal/mtls/` iterates instances, not plugin slugs.

### 6. Remove setup.go's auth-plugin coupling

Source: config-inversion §[catalog deletion].

Rationale: the catalog reshape archived `auth/` (and `security/`). Two auth-coupled paths in `localmesh_src/cmd/localmesh/setup.go` are now dead:

- The `dexseed.Seed(samplePath, ...)` call at the bottom of `runE` reads `localmesh/service_catalog/auth/dex.yaml.sample`, which no longer exists at that path. `make setup` will fail at this step.
- The `LOCALMESH_OIDC_CLIENT_SECRET` mint-once-preserve emission in `envwriter.WriteManaged` exists solely so `dexseed` has a stable secret to read. With `dexseed` gone, this mint becomes dead.

Touches:

- `localmesh_src/cmd/localmesh/setup.go` — drop the dexseed step + the `internal/dexseed` import.
- `localmesh_src/internal/envwriter/envwriter.go` — drop the OIDC mint block.
- `localmesh_src/internal/dexseed/` — consider removing the package entirely once no caller remains.

If a future auth plugin re-promotes from `archive/legacy_plugins/auth/`, those pieces come back together — but as a fresh design, not via a stub-restore.

### 7. Postgres dashboard re-port

Source: `docs/superpowers/specs/2026-05-22-postgres16-plugin-design.md` Out section.

Rationale: the `postgres16/` plugin landed without its `postgres.json` dashboard contribution. The contribution path (per-plugin `configs:` attachment, mount target under `/etc/grafana/dashboards/<plugin>/<slug>.json`) requires the observability plugin to switch its dashboard provider to `foldersFromFilesStructure: true` and migrate the existing `./grafana/dashboards/*` bind mount to per-plugin `configs:` attachments. Sanctioned by DESIGN_DECISIONS row 30. Hold until that observability migration lands.

Touches:
- `localmesh/service_catalog/observability/grafana/provisioning/dashboards/dashboards.yaml` — flip `foldersFromFilesStructure` to `true`.
- `localmesh/service_catalog/observability/docker-compose.yml.gotmpl` — drop the `./grafana/dashboards:/etc/grafana/dashboards:ro` bind mount in favor of per-plugin `configs:` attachments.
- `localmesh/service_catalog/postgres16/postgres.json` — re-port from `git show 8b9feba:localmesh/service_catalog/postgres16/postgres.json` (patched community 9628 + slow-query panel).
- `localmesh/service_catalog/postgres16/docker-compose.yml` — re-declare `grafana:` with `configs:` attachment for `postgres16_dashboard_postgres`.
- `localmesh/service_catalog/postgres16/README.md` — note dashboard is shipped, location, default folder.

## Closed

(none yet)
