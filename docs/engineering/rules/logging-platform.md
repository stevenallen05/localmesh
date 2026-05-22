# Logging platform — omakase

## 0. Data-source rule (highest priority)

**I will define the variables and files you are permitted to pull datapoints from. If I have not told you explicitly where to get the data, you must stop immediately.**

When data isn't where you expected, do not "fix" the gap by reaching for a different input. Do not scrape a label meant for another consumer; do not parse a runtime artifact at build time; do not introduce a new ingestion path to make the call site work. Stop, surface the question, and wait for direction.

This rule is repeated in `CLAUDE.md`, `docs/engineering/rules/golang-basics.md`, `docs/engineering/rules/plugin-conventions.md`, and `docs/engineering/rules/rust-basics.md`.

---

The cross-language logging contract for LocalMesh. Stack-agnostic envelope + cardinality rules + the agent's job; prescriptive per-language emitter and per-source extractor recipes so a team can install one package, copy a 5-line init, and be conformant.

Sibling rules: OTel-semconv (traces + metrics), [`katenary-top-seven.md`](./katenary-top-seven.md) (compose→k8s), [`rust-basics.md`](./rust-basics.md) (stack notes), AUTH / SPIFFE-SPIRE (identity, planned). The compose realisation of this rule lives in [`docs/superpowers/specs/2026-05-17-logging-platform-contract-design.md`](../../superpowers/specs/2026-05-17-logging-platform-contract-design.md).

---

## 1. The event envelope

Every app, every language. **One JSON object per stdout line. NDJSON. No multi-line messages.** This is the only thing the app owns.

| Field | Required | Format | Purpose |
|---|---|---|---|
| `timestamp` | yes | RFC 3339 UTC, ms precision | When the event happened, on the app's clock |
| `level` | yes | lowercase enum: `trace`/`debug`/`info`/`warn`/`error`/`fatal` | Routes alerts, drives default filters, drives retention/sampling tiers |
| `message` | yes | short free-text string | Human-readable. **Not** for structured data |
| `trace_id` | when an OTel span is active | W3C lowercase hex, no dashes | Pivot from log → trace |
| `span_id` | when an OTel span is active | W3C lowercase hex, no dashes | Pivot from log → span |
| `<extras>` | optional | typed kv (string/number/bool/object) | Event-specific structured data: `user_id`, `dashboard_id`, `row_count`, etc. |

Example:

```json
{"timestamp":"2026-05-17T08:34:12.418Z","level":"info","message":"dashboard updated","trace_id":"0af7651916cd43dd8448eb211c80319c","span_id":"b7ad6b7169203331","dashboard_id":"42","user_email":"alice@example.com"}
```

**What is NOT in the envelope:** `service.name`, `service.namespace`, `service.instance.id`, `deployment.environment.name`, `host.name`, `container.id`, `k8s.pod.uid`, `module_name`, `owned_by`. These are *resource* attributes — properties of the source, not the event — and the **agent supplies them** (§3). Apps that emit them anyway aren't wrong; they're just doing work the agent will redo.

## 2. The cardinality contract

The single most common way logging deployments fail in production. Get this wrong and either the backend OOMs (Loki, Prometheus-flavoured stores) or query cost spirals (Splunk, Datadog).

Three tiers. The same rule applies to every backend:

| Tier | Cardinality target | Examples | Where it lives |
|---|---|---|---|
| **Indexed labels** | tens–low hundreds of distinct values per label | `service.name`, `level`, `deployment.environment.name`, `module_name`, `owned_by` | Loki labels, Datadog tags, Splunk indexed fields |
| **Structured metadata / attributes** | unbounded; queryable post-label-selection | `trace_id`, `span_id`, `user_id`, `request_id`, `dashboard_id` | Loki structured metadata, Datadog log attributes, Splunk event fields |
| **Body** | irrelevant — not addressable except by full-text scan | the `message` itself | Loki line, `_raw` |

**Decision test:** *if you'd group-by it on a dashboard, it's a label. If you'd filter for one specific value, it's metadata. If you'd just read it, it's body.*

Anti-patterns the platform punishes:
- PII in indexed labels (GDPR right-to-deletion = re-index nightmare; redact upstream of indexing)
- IDs (user, request, trace, span) as labels — cardinality grows with traffic
- `level` not constrained to the enum — a typo in one emitter becomes a permanent label value
- Body string-concatenation of structured data (`"user 12345 did X"` instead of `{user_id:12345, action:"X", message:"did action"}`)

## 3. The agent's job

The agent (per-node sidecar / daemonset) is where the platform earns its keep. Apps emit the envelope; **the agent does everything else.**

1. **Tail** — read app stdout (docker socket / journald / kubelet log path / cloud log group).
2. **Parse** — extract the JSON body into typed attributes.
3. **Enrich** — add resource attributes the app doesn't know: `service.name`, `service.namespace`, `host.name`, `container.id`, `k8s.pod.uid/name/namespace`, image SHA, cloud region. Source is the orchestrator's metadata, not the app.
4. **Validate** — coerce / drop fields that violate the contract (bound `level` to the enum; reject events without `timestamp`; truncate oversize messages).
5. **Redact** — apply PII patterns to attribute and body strings before egress.
6. **Sample / drop** — by level, by source, by attribute match. Debug logs often sampled or dropped at the edge.
7. **Promote** — selected resource attributes become the backend's indexed labels (§2).
8. **Batch + retry + buffer** — exporter responsibility; never block the app.

**Canonical agents (omakase):**

| Signal | Canonical agent | Why |
|---|---|---|
| Traces, metrics | `opentelemetry-collector(-contrib)` | OTel-native; receivers, processors, exporters for every wire format |
| Logs | `vector` (with `docker_logs` / `kubernetes_logs` source) | Has container/pod-label discovery as a first-class source primitive in both docker and k8s; the otel-collector equivalent exists for k8s (`k8sattributesprocessor`) but not docker, so vector is the portable pick. VRL is also more readable than OTTL for per-source extractor work. |

Either tool can technically do either job; the omakase pick reflects "what's the least painful path that works in both dev compose and prod k8s, today." Teams shouldn't mix agents for the same signal in the same environment.

## 4. Per-language emitter recipe

Each row: install one package + the OTel context bridge, copy a 5–8 line init, emit the envelope.

| Language | Structured logger | OTel context bridge | Status here |
|---|---|---|---|
| **Rust** | `tracing` + `tracing-subscriber` (`json` feature) | `tracing-opentelemetry` | Implemented — see Rust below |
| **Node / TypeScript** | `pino` | manual via `@opentelemetry/api`'s `trace.getActiveSpan()`; or the OTel JS logs SDK once stable | Implemented — see Node below |
| **Go** | stdlib `log/slog` (1.21+) | `go.opentelemetry.io/contrib/bridges/otelslog` | Recipe — unverified |
| **Python** | stdlib `logging` + `python-json-logger` | `opentelemetry-instrumentation-logging` (LoggingInstrumentor) | Recipe — unverified |
| **Java / Kotlin** | `logback` + `logstash-logback-encoder` | `opentelemetry-logback-appender-1.0` | Recipe — unverified |
| **Ruby** | `semantic_logger` | manual via `OpenTelemetry::Trace.current_span` | Recipe — unverified |
| **.NET** | `Microsoft.Extensions.Logging` with JSON console formatter | `OpenTelemetry.Extensions.Logging` | Recipe — unverified |

"Unverified" rows are the right answer in spirit — confirm by emitting one event and grepping for the envelope shape before relying on them in a new service.

### Rust

```toml
# Cargo.toml
tracing = "0.1"
tracing-subscriber = { version = "0.3", features = ["env-filter", "json"] }
tracing-opentelemetry = "0.32"  # bridges tracing events ↔ active OTel span
```

```rust
use tracing_subscriber::{fmt, prelude::*, EnvFilter};

tracing_subscriber::registry()
    .with(EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into()))
    .with(fmt::layer().json().flatten_event(true).with_current_span(true).with_span_list(false))
    .with(tracing_opentelemetry::layer().with_tracer(tracer))
    .init();
```

`tracing-opentelemetry` is what makes `tracing::info!(...)` carry the active OTel span's trace_id/span_id without per-callsite plumbing. Rust's `tracing::Level` serialises uppercase by default (`"INFO"`); the agent lowercases (§3 validate).

### Node / TypeScript

```json
{ "dependencies": { "pino": "^9.5.0", "@opentelemetry/api": "^1" } }
```

```ts
import pino from 'pino';
import { trace } from '@opentelemetry/api';

export const logger = pino({
  formatters: {
    level: (label) => ({ level: label }),  // pino default is numeric
    log: (obj) => {
      const ctx = trace.getActiveSpan()?.spanContext();
      return ctx?.traceId ? { ...obj, trace_id: ctx.traceId, span_id: ctx.spanId } : obj;
    },
  },
  timestamp: () => `,"timestamp":"${new Date().toISOString()}"`,
  messageKey: 'message',
});
```

The OTel JS logs SDK is the future-state replacement for the manual span lookup; not yet stable enough to make the omakase pick.

## 5. Per-source agent recipe

The agent's enrichment job for sources that **don't** emit the envelope themselves (databases, off-the-shelf infra, cloud services). Goal: same envelope at egress, source-specific work at the agent.

| Source | Native format | Recipe |
|---|---|---|
| **PostgreSQL 15+** | configurable | `log_destination = 'jsonlog'` → log lines already JSON; agent maps `error_severity` → `level`, `message` passes through, `query_id` → attribute |
| **PostgreSQL ≤14** | logfmt-ish | Vector `parse_regex` against `log_line_prefix`, then synthesise envelope |
| **nginx (access)** | configurable | `log_format` with JSON template; agent extracts `request_method`/`uri`/`status` as attributes; `status>=500 → level=error` |
| **nginx (error)** | text | regex parse; severity word → `level` enum |
| **Grafana** | logfmt | Vector `parse_logfmt` → already structured; `lvl` → `level` |
| **Loki / Tempo / Mimir / VictoriaMetrics** | logfmt or JSON | varies by version; same pattern — parse, map `level`, set `service.name` from container label |
| **redis** | text with `LOG_LEVEL` prefix | regex; severity letter (`*` / `#` / `-` / `.`) → `level` enum |
| **otel-collector self-logs** | zap JSON | direct field map: `level`, `msg`→`message`, `ts`→`timestamp` |
| **Kafka / ZooKeeper (JVM)** | log4j pattern | Either reconfigure log4j to JSON layout (preferred) or per-pattern regex at the agent |
| **k8s controller manager / kubelet / etc.** | klog JSON when `--logtostderr=true --log-json=true` | Direct field map |
| **Generic JSON-emitting container** | JSON, unknown schema | Parse + field-name normalisation table; carry unknown fields as attributes |

The rule the agent operates by: **at egress, every event has the envelope.** Where the source didn't emit it, the agent synthesises it. Where the source emits something close, the agent renames + coerces.

## 6. What this rule does NOT prescribe

- **Backend.** Loki, Datadog, Splunk, Elastic, Honeycomb, GCP Cloud Logging, AWS CloudWatch Logs — all behind OTLP or a vendor wire format. The contract is portable.
- **Retention.** Per-environment / per-cost-tier; set at the backend.
- **Sampling policy.** Per-environment; set at the agent.
- **Alerting rules.** Live with the team / SRE org that owns the alert, not in the logging platform.
- **Multi-tenancy boundaries.** Backend-specific (`X-Scope-OrgID` for Loki, indexes for Datadog/Splunk). Set at the agent's exporter config.
- **PII patterns.** Org-specific; the agent has a redaction processor slot — what goes in it is policy, not platform.

## 7. Relationship to other LocalMesh contracts

| Contract | Where it lives | Why this rule cares |
|---|---|---|
| OTel-semconv (traces + metrics) | DESIGN_DECISIONS row + spec `2026-05-16-otel-semconv-pipeline-design.md` | Logs carry `trace_id`/`span_id` from the same OTel span context |
| Module attribution | DESIGN_DECISIONS row | `module_name` / `owned_by` are resource attrs the agent supplies from container/pod labels |
| Identity (SPIFFE/SPIRE) | [`docs/AUTH.md`](../../AUTH.md), planned spec | A future iteration of the envelope may add `user.id` as a structured-metadata attribute when an authenticated request is in flight |
| katenary labels | [`katenary-top-seven.md`](./katenary-top-seven.md) | The compose `metrics.*` label namespace is the source the docker-side agent reads from; `k8s` overlay reads pod labels for the same attrs |
