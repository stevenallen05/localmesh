# Consumer wiring — `logging/`

## Overview

Vector tails the docker socket and ships logs to Loki. Apps participate by emitting one JSON object per stdout line — Vector enriches identity from container labels, normalises severity, validates the envelope, and forwards. The wiring is a format contract, not a config wire-up: no env, no volumes, no `depends_on`.

## Environment

None.

## Volumes

None.

## depends_on

None. Vector reads via the docker socket; startup ordering does not matter, and consumers do not block on Vector being up.

## Labels

The identity-tuple labels every service in this project already carries (per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md) §1) are the wiring for logging:

```yaml
labels:
  metrics.service_name: <your-service>
  metrics.module_name: <your-module>     # `app` for app-tier
  metrics.owned_by: <contact-email>
```

Vector reads these from container labels and stamps every log event with the same identity used on traces and metrics. Without them the log events still reach Loki but lose their identity, breaking the cross-signal pivot.

## k8s rendering

None. The k8s overlay swaps Vector's `docker_logs` source for `kubernetes_logs`; the consumer-facing contract (labels + JSON envelope) is unchanged.

## Notes

- **JSON envelope contract.** Full contract: [`../../docs/engineering/rules/logging-platform.md`](../../docs/engineering/rules/logging-platform.md). One object per stdout line: `timestamp` (RFC 3339 UTC ms), `level` (lowercase enum: `trace`/`debug`/`info`/`warn`/`error`/`fatal`), `message`, `trace_id` + `span_id` (lowercase hex W3C, when an OTel span is active), plus free-form attributes.
- **Resource attrs are NOT in the envelope.** Apps must not emit `service.name`, `module_name`, `owned_by` inside the JSON — Vector enriches from the labels above. Duplicates create two channels of truth.
- **Per-language emitter recipes.** The logging-platform rules doc includes drop-in setups for Rust (`tracing-subscriber` + `tracing-opentelemetry`), Node (Pino + `@opentelemetry/api`), and other stacks.

## Example app service block

```yaml
services:
  myservice:
    labels:
      metrics.service_name: myservice
      metrics.module_name: app
      metrics.owned_by: ${TECH_LEAD_EMAIL}
```

The app emits JSON to stdout following the envelope contract; nothing else compose-side.
