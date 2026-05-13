# Production decision matrix

A **structured research scaffold** for evaluating candidate solutions
against consistent dimensions, used in conjunction with
[`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md).

> **This document is intentionally blank.** Filling in the cells with
> real comparative research is **beyond the scope of this take-home**.
> The framework is the deliverable; a production engagement does the
> research and fills in the cells.

## How to read

Each decision evaluates candidate solutions across **up to 7 dimensions**
using a **3-star rating scale**:

| Rating | Meaning | What it implies (≤10 words) |
|---|---|---|
| ★☆☆ | **No** | Solution doesn't meaningfully address this dimension. |
| ★★☆ | **Sorta** | Partial fit; works with explicit caveats or effort. |
| ★★★ | **Yes** | Solution fits this dimension naturally, low friction. |

**Conventions:**

- Candidate lists are **examples, not exhaustive** — real research
  surfaces the real candidates.
- Dimensions are **chosen per decision** — pick the ≤7 that matter most
  for the organization's situation; don't force every decision into the
  same shape.
- Empty cells are correct at design time. They get filled in by real
  engineering investigation.

---

## Example: service mesh

Underlying conversation:
[`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md) §Security,
compliance & identity → *"How does a service prove its identity to
another?"*

**Candidate solutions** (non-exhaustive):
Istio, Linkerd, Cilium Service Mesh, Consul Connect, AWS App Mesh.

| Dimension | Istio | Linkerd | Cilium | Consul Connect | AWS App Mesh |
|---|---|---|---|---|---|
| Low operational complexity | | | | | |
| Workload-identity (SPIFFE) integration | | | | | |
| Multi-cluster / multi-region support | | | | | |
| Documentation maturity | | | | | |
| Community size & activity | | | | | |
| Total cost of ownership | | | | | |
| Existing team skill fit | | | | | |

---

## Using this template for other decisions

Each major decision from
[`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md) gets its own
matrix. A real engagement copies the structure above with dimensions and
candidates relevant to the decision. Some likely starting points:

| Decision | Underlying conversation | Example candidates (non-exhaustive) |
|---|---|---|
| **Secrets backend** | Security & identity → "Where do secrets live?" | HashiCorp Vault, External Secrets Operator, Sealed Secrets, AWS Secrets Manager, GCP Secret Manager, Doppler |
| **Time-series database** | Observability hardening + Storage | Prometheus, VictoriaMetrics, Mimir, Thanos, InfluxDB, ClickHouse |
| **Logs aggregation backend** | Observability hardening → "Where do logs go?" | Loki, ELK, ClickHouse, Datadog, Splunk |
| **Tracing backend** | Observability hardening → "Where do traces go?" | Tempo, Jaeger, Honeycomb, Datadog APM |
| **Rollback tooling tier** | Reliability → "How do teams roll back?" | Manual (kubectl rollout undo), GitOps-driven (Argo CD / Flux), Progressive delivery (Argo Rollouts / Flagger), Vendor-managed (Spinnaker / Harness) |
| **Image registry** | CI/CD → "Supply-chain security" | Harbor, GHCR, ECR, GCR, ACR, Quay, Artifactory |
| **CI/CD orchestrator** | CI/CD → "Promotion path" + "Trigger conventions" | GitHub Actions, GitLab CI, Tekton, Argo Workflows, Buildkite, CircleCI |
| **Dashboard provisioning model** | Observability hardening → "Dashboards as code vs. UI" | Grafana (UI-edited), Grafana (provisioned via dashboards-as-code), Perses, vendor-managed |
| **Cluster ingress** | Networking → "Ingress strategy" | NGINX Ingress, Traefik, Istio Gateway, Envoy Gateway, cloud-vendor LB |

For each: pick up to 7 dimensions, list the real candidates, leave the
cells empty for the research phase.
