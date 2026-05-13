# Tensorwave take-home

> A reference implementation of the **microservice-template + service-catalogue**
> deployment pattern, demonstrated through a small Rust + Next.js metrics service.

The take-home asked for a metrics monitoring system. The artifact here is
*one team's microservice repo* shaped the way an organization's CI/CD core
should support it: a top-level `docker-compose.yml` that `include:`s an
observability module from a stub service catalogue. Helm charts are
generated from compose, not hand-maintained.

## Read these first

- [`docs/ENGINEERING_RULES.md`](./docs/ENGINEERING_RULES.md) — the design
  rules and assumptions underlying every decision in the repo. Each rule
  is tied to a business need and stays generic; no project-specific
  references. Read these to understand the *why*.
- [`docs/PROJECT_SCOPE.md`](./docs/PROJECT_SCOPE.md) — the boundary
  between what this take-home actually ships and what a production
  deployment would assume. Read this before treating any "PoC shortcut"
  or "dev-only" caveat as a gap.
- [`docs/DESIGN_DECISIONS.md`](./docs/DESIGN_DECISIONS.md) — current state
  of every design choice, one row each. The dev/rationale/prod columns
  give you the shape of the system in a couple of minutes.

## Project documentation

- [`requirements.md`](./requirements.md) — original take-home requirements (read-only reference).
- [`docs/ENGINEERING_RULES.md`](./docs/ENGINEERING_RULES.md) — north-star design rules. Generic; tied to business needs.
- [`docs/PROJECT_SCOPE.md`](./docs/PROJECT_SCOPE.md) — dev/prod scope boundary.
- [`docs/DESIGN_DECISIONS.md`](./docs/DESIGN_DECISIONS.md) — current shape of each design choice.
- [`docs/rules/`](./docs/rules/) — stack-specific style notes (Rust, katenary).
- [`docs/superpowers/`](./docs/superpowers/) — exploration artifacts, design specs, plans, and other agentic/research material. Detailed technical specs live under `docs/superpowers/specs/`.
