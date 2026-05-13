In rough order of "if you skip this, your chart is silently broken":

**1. How to `build:` with `image:` tags**

Katenary references images from a registry; it can't run a build. `build: .` will either error or produce a chart pointing at nothing useful. Always use `image: registry/repo:explicit-tag` — not `:latest`, not bare names. The tag becomes part of the Helm chart's deploy contract.

> *PoC shortcut:* hardcoded `0.1.0`, no registry prefix. Prod evolution in [`PROJECT_SCOPE.md`](../../stakeholder/PROJECT_SCOPE.md).

**2. `katenary.v3/main-app` on exactly one service**

Marks which service represents the application as a whole. Drives `Chart.yaml` `appVersion`, the chart name's identity, and image-tag overrides in `values.yaml`. Skip it and you get sensible-ish defaults that bite you on the first upgrade because the chart version and app version drift.

```yaml
webapp:
  labels:
    katenary.v3/main-app: "true"
```

> *PoC shortcut:* single chart with `server` as main-app. Per-service charts (Conway's-law shaped) are a [`PROJECT_SCOPE.md`](../../stakeholder/PROJECT_SCOPE.md) item.

**3. `katenary.v3/ports` on every `depends_on` target**

Compose `depends_on` becomes an init container that TCP-probes the dependency. The probe needs a port. If the target service doesn't declare `ports:` or `expose:`, the init container has nothing to wait on and either fails generation or — worse — generates a broken probe that hangs the pod indefinitely.

```yaml
database:
  labels:
    katenary.v3/ports: |-
      - 5432
```

> *PoC shortcut:* port label only; no real readiness probe. Prod pairs with `katenary.v3/health-check` — see [`PROJECT_SCOPE.md`](../../stakeholder/PROJECT_SCOPE.md).

**4. `katenary.v3/map-env` for any cross-service hostname**

Katenary prefixes every service with `{{ .Release.Name }}-` so multiple releases can coexist in one namespace. Your `DB_HOST: database` still says `database` in the rendered chart and won't resolve. This one always bites teams porting from compose because everything *looks* right in the YAML.

```yaml
webapp:
  environment:
    DB_HOST: database
  labels:
    katenary.v3/map-env: |-
      DB_HOST: "{{ .Release.Name }}-database"
```

> *PoC shortcut:* cross-service hostnames rewritten inline per service. Multi-release setups should switch to a shared ConfigMap of endpoints.

**5. `katenary.v3/secrets` for sensitive env vars**

By default everything in `environment:` lands in a ConfigMap. Database passwords, API keys, anything you care about — explicitly flag them or you'll ship plaintext creds in a ConfigMap and fail any reasonable audit. This is the most common "we deployed for months and nobody noticed" footgun.

```yaml
database:
  environment:
    POSTGRES_PASSWORD: changeme
  labels:
    katenary.v3/secrets: |-
      - POSTGRES_PASSWORD
```

> *PoC shortcut:* label is set, but the dev password sits in compose plaintext. Prod uses a real secrets backend — see [`PROJECT_SCOPE.md`](../../stakeholder/PROJECT_SCOPE.md).

**6. `katenary.v3/ingress` for anything externally reachable**

Compose `ports: 8080:80` becomes a ClusterIP Service, not an Ingress. There is no automatic translation — k8s has no opinion about whether port-mapping means "expose to the internet." Declare it explicitly or your service is only reachable inside the cluster.

```yaml
webapp:
  labels:
    katenary.v3/ingress: |-
      hostname: myapp.example.com
      port: 80
```

> *PoC shortcut:* placeholder hostname `www.example.com` baked in. Prod uses `values.yaml` override + cert-manager — see [`PROJECT_SCOPE.md`](../../stakeholder/PROJECT_SCOPE.md).

**7. `katenary.v3/ignore` on every dev-only service**

Mailhog, adminer, mailcatcher, swagger-ui sidecars, debug proxies — anything that exists in compose for local-dev convenience must be flagged or it goes to prod. Katenary has no way to guess "this one's just for me." Add the label at the moment you add the service, not later.

```yaml
mailhog:
  image: mailhog/mailhog:v1.0.1
  labels:
    katenary.v3/ignore: "true"
```

> *PoC shortcut:* nothing to label — no dev-only services yet. Add the label in the same edit that introduces the first one.

**Honorable mentions** worth knowing about once the seven above are habit:

`katenary.v3/values-from` to avoid duplicating creds across services; `katenary.v3/configmap-files` instead of bind-mounting static config (and never bind-mount source — that doesn't translate at all); named volumes rather than host paths for anything that needs persistence (they become PVCs, host paths become a mess); `katenary.v3/health-check` so the chart ships real readiness/liveness probes rather than k8s's "process is alive = service is ready" default, which is almost never what you want.