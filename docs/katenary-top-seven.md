In rough order of "if you skip this, your chart is silently broken":

**1. How to `build:` with `image:` tags**

Katenary references images from a registry; it can't run a build. `build: .` will either error or produce a chart pointing at nothing useful. Always use `image: registry/repo:explicit-tag` — not `:latest`, not bare names. The tag becomes part of the Helm chart's deploy contract. Build context

> *PoC shortcut:* hardcoded `0.1.0` placeholder, no registry prefix, no CI tag derivation. Prod CI sets the tag from a git release and pushes to a real registry (`ghcr.io/...`, `<account>.dkr.ecr...`).

**2. `katenary.v3/main-app` on exactly one service**

Marks which service represents the application as a whole. Drives `Chart.yaml` `appVersion`, the chart name's identity, and image-tag overrides in `values.yaml`. Skip it and you get sensible-ish defaults that bite you on the first upgrade because the chart version and app version drift.

```yaml
webapp:
  labels:
    katenary.v3/main-app: "true"
```

> *PoC shortcut:* single chart, `server` is main-app. Prod splits each service into its own per-service chart (Conway's law) and this top-level chart becomes a meta-chart that depends on them.

**3. `katenary.v3/ports` on every `depends_on` target**

Compose `depends_on` becomes an init container that TCP-probes the dependency. The probe needs a port. If the target service doesn't declare `ports:` or `expose:`, the init container has nothing to wait on and either fails generation or — worse — generates a broken probe that hangs the pod indefinitely.

```yaml
database:
  labels:
    katenary.v3/ports: |-
      - 5432
```

> *PoC shortcut:* port label only; no actual readiness probe behind it. Prod pairs this with `katenary.v3/health-check` so the init container waits on real readiness rather than TCP-open.

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

> *PoC shortcut:* one cross-service hostname (`www → server`), rewritten inline. Sufficient for the single-release demo; once multiple releases or namespaces are in play, prefer a shared ConfigMap of service endpoints over duplicating these labels.

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

> *PoC shortcut:* label is set, but the dev password (`postgres`) sits in compose plaintext. Prod replaces it with an overlay that wires the chart's Secret to External Secrets / Sealed Secrets / Vault.

**6. `katenary.v3/ingress` for anything externally reachable**

Compose `ports: 8080:80` becomes a ClusterIP Service, not an Ingress. There is no automatic translation — k8s has no opinion about whether port-mapping means "expose to the internet." Declare it explicitly or your service is only reachable inside the cluster.

```yaml
webapp:
  labels:
    katenary.v3/ingress: |-
      hostname: myapp.example.com
      port: 80
```

> *PoC shortcut:* placeholder hostname `www.example.com` baked in. Prod overrides via `values.yaml` and pairs with cert-manager (or the cluster's standard TLS issuer) for the actual certificate.

**7. `katenary.v3/ignore` on every dev-only service**

Mailhog, adminer, mailcatcher, swagger-ui sidecars, debug proxies — anything that exists in compose for local-dev convenience must be flagged or it goes to prod. Katenary has no way to guess "this one's just for me." Add the label at the moment you add the service, not later.

```yaml
mailhog:
  image: mailhog/mailhog:v1.0.1
  labels:
    katenary.v3/ignore: "true"
```

> *PoC shortcut:* nothing to label — no dev-only services in compose yet. When the first one lands (adminer, mailhog, a debug proxy), add the label in the same edit that adds the service.

**Honorable mentions** worth knowing about once the seven above are habit:

`katenary.v3/values-from` to avoid duplicating creds across services; `katenary.v3/configmap-files` instead of bind-mounting static config (and never bind-mount source — that doesn't translate at all); named volumes rather than host paths for anything that needs persistence (they become PVCs, host paths become a mess); `katenary.v3/health-check` so the chart ships real readiness/liveness probes rather than k8s's "process is alive = service is ready" default, which is almost never what you want.