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

### 2. postgres16 adopts `config_vars` + `exports`

Source: config-inversion §11.2.

Rationale: postgres still hardcodes `POSTGRES_DB`, `POSTGRES_USER`, `container_name`, and a README-documented `DATABASE_URL`. The inversion lets the plugin own assembly and the consumer name the destination. Held off Phase 1 because the mTLS connection string is gnarly (`postgresql://server@<host>:<port>/<db>?sslmode=verify-full&sslcert=…&sslkey=…`). Also folds in the pre-existing `project.toml` slug mismatch (`plugins = [..., "postgres", ...]` while the directory is `postgres16/`); the adoption pass renames consistently.

Touches:

- `localmesh/service_catalog/postgres16/plugin.toml` — add `[[config_vars]]` (`database_name`, role names) + one `[[exports]]` (`connection_url`).
- `localmesh/service_catalog/postgres16/docker-compose.yml` — read `POSTGRES_DB` / `POSTGRES_USER` from `${POSTGRES16_*}` interpolation.
- `localmesh/service_catalog/postgres16/README.md` — Environment row points at the `deploy.toml`-chosen destination.

### 3. Per-plugin README Environment table

Source: config-inversion §11.3.

Rationale: once `exports` is in place the Environment table duplicates manifest data. Either auto-generate from exports or trim the section to a pointer at the consumer's resolved destination.

Touches:

- `docs/engineering/rules/plugin-conventions.md` §3 — Environment subsection rules.
- Every plugin's `README.md` — once §3 lands.

### 4. `plugin-conventions.md` §4 env-var naming refresh

Source: config-inversion §11.4.

Rationale: §4 documents `<PLUGIN>_*` and `<CONTAINER>_*` patterns for envwriter-emitted vars. Phase 1 adds `<PLUGIN>_<CONFIG>` for config_var-driven internal env vars and dev-chosen names for consumer-facing exports. The section needs the new rules once Phase 1 lands.

Touches: `docs/engineering/rules/plugin-conventions.md` §4.

### 5. `project.toml` ↔ `deploy.toml` sync

Source: config-inversion §11.5.

Rationale: a plugin's slug appears in both files (selection vs instantiation). Phase 1 keeps them in manual sync. Phase 3 (`localmesh add`) makes it automatic. Until then a manifest-loader cross-check could warn on "selected but never instantiated" (the reverse — instantiated but not selected — is already caught by `LoadDeploy` validation).

Touches: `localmesh_src/internal/manifest/manifest.go` cross-validation; eventually a `localmesh add` verb.

### 6. Identity tuple rebind (Phase 2 trigger)

Source: config-inversion §11.6 (and Phase 2 in §8).

Rationale: multi-instance support requires `service_name` to become the per-instance OTel `service.name` and per-instance container name; `module_name` stays the plugin-kind axis. The current convention (`service_name` = container slug, `module_name` = plugin slug, one-per-container-role) doesn't survive two instances of the same source.

Touches:

- `docs/engineering/rules/plugin-conventions.md` §2 — new binding.
- Every plugin's `docker-compose.yml` once it adopts per-instance rendering — moves from plain `.yml` to `.gotmpl` keyed by `service_name`.
- Per-instance cert minting — `localmesh_src/internal/mtls/` iterates instances, not plugin slugs.

## Closed

(none yet)
