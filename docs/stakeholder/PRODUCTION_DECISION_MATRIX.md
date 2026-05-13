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

- **Dimensions are 1-2 words.** Sharp axes beat verbose ones.
- **Candidate lists** are examples, not exhaustive — real research
  surfaces the real candidates.
- **Empty cells are correct at design time.** They get filled in by real
  engineering investigation.
- **Pick up to 7 dimensions per decision** — don't force every decision
  into the same shape.

---

## Example: choosing a cluster OS (bare-metal)

The shape of a bare-metal Kubernetes deployment is set by which OS
underpins the cluster. Underlying conversation:
[`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md) §Reliability,
scale & multi-region → *"How are hardware drivers versioned and
updated?"* + §Provider & cost → *"Cloud, on-prem, or hybrid?"*

**Candidate solutions** (non-exhaustive):
TalosOS, RHEL CoreOS, Ubuntu Server LTS, Rocky Linux, openSUSE Leap Micro.

| Dimension | TalosOS | RHEL CoreOS | Ubuntu LTS | Rocky | openSUSE |
|---|---|---|---|---|---|
| Immutable | | | | | |
| API surface | | | | | |
| Hardware breadth | | | | | |
| Driver story | | | | | |
| Air-gap | | | | | |
| Maturity | | | | | |
| Cost | | | | | |

---

## Using this template for other decisions

Each major decision from
[`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md) gets its own
matrix. Likely starting points, oriented toward a bare-metal /
datacenter deployment:

| Decision | Conversation | Example candidates (non-exhaustive) |
|---|---|---|
| **Cluster OS** | (example above) | TalosOS, RHEL CoreOS, Ubuntu LTS, Rocky, openSUSE Micro |
| **CNI / pod networking** | Networking → "ingress strategy" | Cilium, Calico, Flannel, Weave |
| **Bare-metal load balancer** | Networking → "ingress strategy" | MetalLB, Cilium LB, Kube-VIP |
| **Distributed storage** | Storage → "named storage classes" | Longhorn, Rook/Ceph, OpenEBS, Portworx, MayaStor |
| **Hardware lifecycle / provisioning** | Reliability → "driver versioning" | Tinkerbell, Metal3, Rancher Elemental, vendor BMC tooling |
| **Service mesh** | Security → "service identity" | Istio, Linkerd, Cilium SM, Consul Connect |
| **Operational isolation tier** | Security → "customer vs. company isolation" | Shared cluster, dedicated namespaces, dedicated node pools, dedicated clusters, physical separation |
| **Secrets backend** | Security → "where do secrets live" | Vault, External Secrets Operator, Sealed Secrets, HSM-backed |
| **Logs aggregation** | Observability → "where do logs go" | Loki, ELK, ClickHouse, vendor (Datadog, Splunk) |
| **Tracing backend** | Observability → "where do traces go" | Tempo, Jaeger, Honeycomb |
| **Rollback tooling tier** | Reliability → "how do teams roll back" | Manual, GitOps (Argo CD / Flux), Progressive delivery (Argo Rollouts / Flagger), Vendor-managed |
| **Image registry** | CI/CD → "supply-chain security" | Harbor, GHCR, ECR, GCR, Quay, Artifactory |
| **CI/CD orchestrator** | CI/CD → "promotion path" | GitHub Actions, GitLab CI, Tekton, Argo Workflows, Buildkite |
| **Dashboard provisioning** | Observability → "dashboards as code" | Grafana (UI-edited), Grafana (provisioned), Perses, vendor-managed |

For each: pick up to 7 sharp dimensions, list real candidates, leave the
cells empty for the research phase.
