# Production discussions

Conversation categories a production engagement of this architecture would surface. Thread-starters, not answers — depth depends on context.

Companion: [`PRODUCTION_DECISION_MATRIX.md`](./PRODUCTION_DECISION_MATRIX.md) shows the visual shape vendor-choice deliberations take.

- **Identity & access boundary.** Observability access, service-to-service authentication, customer/company workload isolation.
- **Compliance posture.** Top-level `x-katenary:` flags in each team's `docker-compose.yml` (e.g. `stores_pii_data: true`, `stores_pci_data: true`) flowing automatically into backup, retention, and audit rules applied across every service in the stack.
- **Catalogue governance.** Versioning, deprecation, the severity model for blocking-vs-warning rules, migration runway.
- **Build vs. buy.** Per-module managed / self-hosted choice; chargeback; per-team quotas.
- **Handling exceptions.** The workloads that don't fit a catalogue rule, default, or version pin. The right answer depends on the *why*.
