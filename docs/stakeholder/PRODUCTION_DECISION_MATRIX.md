# Production decision matrix

A **structured research scaffold** for evaluating candidate solutions
against consistent dimensions, used in conjunction with
[`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md).

> **This document is intentionally blank.** Filling in the cells with
> real comparative research is **beyond the scope of this take-home**.
> The framework is the deliverable; a production engagement does the
> research and fills in the cells.

This matrix is the operational tool for
[`ENGINEERING_RULES.md`](./ENGINEERING_RULES.md) **Rule 7 — defer
over-determined choices**. The decisions captured here are the ones that
fit into the *"TBD until we know more about prod"* category — vendor
and tooling choices that depend on organizational context (SCM in use,
existing skill base, cloud relationships, customer commitments) that
doesn't yet exist or hasn't yet been surfaced.

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

## Example: choosing a CI/CD orchestrator

CI/CD orchestrator choice is the canonical *"TBD until we know more"*
decision: it depends on where source code lives (which SCM), the org's
existing CI footprint, deployment shape (Kubernetes-native or not), and
team skill base. Underlying conversation:
[`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md) §CI/CD &
release management → *"What's the promotion path?"* and *"What triggers
a deploy?"*

**Candidate solutions** (non-exhaustive):
GitHub Actions, GitLab CI, Tekton, Argo Workflows, Buildkite.

| Dimension | GH Actions | GitLab CI | Tekton | Argo Workflows | Buildkite |
|---|---|---|---|---|---|
| Self-hosted | | | | | |
| K8s-native | | | | | |
| Cost | | | | | |
| Maturity | | | | | |
| Integrations | | | | | |
| Speed | | | | | |
| Skill fit | | | | | |

---

## Using this template for other decisions

Each major *defer-until-prod-context* decision from
[`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md) gets its own
matrix. Likely starting points, in roughly the order they tend to come
up in a production engagement:

| Decision | Conversation | Example candidates (non-exhaustive) |
|---|---|---|
| **CI/CD orchestrator** | (example above) | GitHub Actions, GitLab CI, Tekton, Argo Workflows, Buildkite, Jenkins |
| **External load balancer** | Networking → "ingress strategy" | NGINX, HAProxy, Envoy, Traefik, Istio Gateway, cloud ALB/GLB |
| **Static content / CDN** | Networking → "how do static assets get to users" | CloudFront, Cloudflare, Fastly, Akamai, Bunny, KeyCDN |
| **Image registry** | CI/CD → "supply-chain security" | Harbor, GHCR, ECR, GCR, Quay, Artifactory |
| **DNS provider** | Networking (cert routing + ingress) | Cloudflare DNS, Route53, NS1, dnsimple |
| **WAF / DDoS protection** | Networking + Security | Cloudflare, AWS WAF, Akamai, Imperva |
| **SSO / IdP** | Security → "operator auth & audit" + "embedded-dashboard auth" | Okta, Azure AD, Auth0, Keycloak, Google Workspace |
| **Secrets backend** | Security → "where do secrets live" | Vault, External Secrets Operator, Sealed Secrets, AWS Secrets Manager, Doppler |
| **Service mesh** | Security → "service identity" | Istio, Linkerd, Cilium SM, Consul Connect |
| **Logs aggregation** | Observability → "where do logs go" | Loki, ELK, ClickHouse, Datadog, Splunk |
| **Tracing backend** | Observability → "where do traces go" | Tempo, Jaeger, Honeycomb, Datadog APM |
| **Dashboard provisioning** | Observability → "dashboards as code vs. UI" | Grafana (UI), Grafana (provisioned), Perses, vendor-managed |
| **Rollback tooling tier** | Reliability → "how do teams roll back" | Manual, GitOps (Argo CD / Flux), Progressive delivery (Argo Rollouts / Flagger), Vendor-managed |
| **Email / notification gateway** | Cross-cutting | SES, SendGrid, Postmark, Mailgun, vendor SMTP |
| **Bare-metal cluster stack** | Provider & cost → "cloud, on-prem, or hybrid" | TalosOS + MetalLB + Longhorn (bare-metal); EKS / GKE / AKS (cloud); Rancher (hybrid) |

For each: pick up to 7 sharp dimensions, list real candidates, leave the
cells empty for the research phase.
