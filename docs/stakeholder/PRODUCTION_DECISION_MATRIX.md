# Production decision matrix

Visual shape for the kind of decision-framework needed once production context is known. Operational tool for Rule 7 of [`ENGINEERING_RULES.md`](./ENGINEERING_RULES.md) — **defer over-determined choices**.

> **Intentionally blank.** Filling cells with real research is beyond this take-home's scope; the framework is the deliverable.

3-star ratings, up to 7 dimensions per decision. Dimensions are 1-2 words. Candidate lists are illustrative.

| Rating | Meaning |
|---|---|
| ★☆☆ | **No.** Doesn't address this dimension. |
| ★★☆ | **Sorta.** Partial fit with caveats. |
| ★★★ | **Yes.** Fits naturally. |

## Example: Secrets backend

Replaces the dev `secrets/` directory and `localmesh ca mint` keys. Pairs with the **Local mTLS (mutual TLS) setup** and **Secrets injection** rows in [`DESIGN_DECISIONS.md`](./DESIGN_DECISIONS.md). See [`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md).

**Candidates** (a mix of self-hosted strategies and managed vendors): Vault, External Secrets Operator (ESO), Sealed Secrets, AWS Secrets Manager (AWS SM), Doppler.

| Dimension | Vault | ESO | Sealed Secrets | AWS SM | Doppler |
|---|---|---|---|---|---|
| Self-hosted | | | | | |
| Kubernetes-native | | | | | |
| Rotation | | | | | |
| Audit trail | | | | | |
| Cloud-agnostic | | | | | |
| Operator burden | | | | | |
| Skill fit | | | | | |

Every other defer-until-prod decision (CI/CD orchestrator, load balancer, CDN (Content Delivery Network), image registry, DNS, service mesh, observability backends, rollback tooling, IdP (Identity Provider), bare-metal stack, …) gets the same treatment.
