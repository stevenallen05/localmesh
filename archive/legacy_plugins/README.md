# `archive/legacy_plugins/`

Plugins from the pre-inversion catalog. They're kept here for reference. The CLI does not scan this directory — `localmesh build` only looks under `localmesh/service_catalog/<name>/`, so nothing here participates in the live build.

| Plugin | Why it's here |
|---|---|
| `auth/` | Dex IdP plugin. Holds the dex.yaml.sample + OIDC client glue the pre-inversion stack relied on. Out of date relative to the inverted-config shape; useful as a reference if a future Dex or OAuth-like plugin needs to land. |
| `security/` | Kuma-based mTLS plugin (ingress + sidecar + egress). Documents how cross-service mTLS used to be expressed before the catalog was reshaped. Out of date — the next mesh integration may be Istio / Linkerd / Cilium — but the bootstrap configs, sidecar Dockerfile, and policy templates are still informative. |

Other pre-inversion plugins (`base/`, `postgres16/`) were removed outright in the same change. `base/` was just a meta-plugin reference list; `postgres16/` needs the inversion's `config_vars` + `exports` machinery before it can rejoin the live catalog (tracked in `docs/CLEANUP.md` §2).

If a plugin is re-promoted, copy (not move) the directory back under `localmesh/service_catalog/` and bring it up to the current `plugin.toml` schema (`[[config_vars]]` + `[[exports]]` per `docs/engineering/rules/plugin-conventions.md` §4).
