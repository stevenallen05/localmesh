# `inverting_configs/` — seed pack for the plugin config inversion

Snapshot of the brainstorming + spec + plan + moving-target files produced during the design pass for inverting plugin configuration. Captures the agreed shape so a fresh session can pick the work up without re-deriving everything.

## What's here

| File | Role |
|---|---|
| `spec.md` | The design spec (original at `docs/superpowers/specs/2026-05-22-plugin-config-inversion-design.md`, gitignored). |
| `plan.md` | The implementation plan (original at `docs/superpowers/plans/2026-05-22-plugin-config-inversion.md`, gitignored). |
| `CLEANUP.md` | Follow-up tracker for items this design parks or creates. Originally lands at `docs/CLEANUP.md`. |
| `project.toml` | Top-of-project file in its inverted-config shape. `plugins = ["redis"]`, single app-tier `[[services]]` for `www`. |
| `deploy.toml` | New file (lives at repo root). Per-instance orders. One live `[[plugins.redis]]` block (`service_name = "cache"`) plus commented Phase 2 multi-instance examples. |
| `docker-compose.yml` | Top-level compose showing the consumer pattern (`www` reads `REDIS_URL: ${CACHE_URL}`). |
| `example.env` | The output target. Shows what `localmesh build` produces in `.env`'s managed section for these inputs. |
| `redis/plugin.toml` | The worked-example plugin manifest. Identity + one `[[services]]` + two `[[config_vars]]` (`maxmemory`, `noevict`) + one `[[exports]]` (`connection_url`). |
| `redis/docker-compose.yml` | The plugin's compose. Stock `redis:7-alpine`, reads `${REDIS_MAXMEMORY}`. |
| `redis/README.md` | Consumer-wiring README per `docs/engineering/rules/plugin-conventions.md` §3 template. |

## How the files reference each other

```
project.toml          plugins = ["redis"]
       │                       │
       │   selects             │   instantiates
       ▼                       ▼
docker-compose.yml      deploy.toml
  www reads               [[plugins.redis]] service_name = "cache"
  ${CACHE_URL}              [plugins.redis.config]   maxmemory = "512mb"
                            [plugins.redis.exports]  connection_url = "CACHE_URL"
                                    │
                                    │   resolved by `localmesh build` against
                                    ▼   redis/plugin.toml's [[config_vars]] + [[exports]]
                            example.env  CACHE_URL=redis://redis:6379
                                         REDIS_MAXMEMORY=512mb
```

## What's intentionally NOT here

- The actual Go code that resolves these (per `plan.md`, the implementation is what the fresh session starts on).
- The other deletions to `localmesh/service_catalog/` (`auth/`, `base/`, `postgres16/`, `security/`) — that catalog-reshape is a Chunk 4 commit in the plan, not part of the seed.
- The Go toolchain — required for execution per `CLAUDE.md`, but not part of this snapshot.

## Status of the precursor design

`spec.md` supersedes `docs/superpowers/specs/2026-05-22-plugin-env-vars-design.md` (the prefix-forced flat `[[env_vars]]` design that hit four roadblocks; archived elsewhere). Section 2 of `spec.md` records the roadblocks and how the inversion clears them.
