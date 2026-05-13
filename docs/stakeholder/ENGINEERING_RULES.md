# Engineering rules

> **North star.** These rules optimize for two organizational outcomes:
> **velocity of product-team shipping** and **architectural coherence
> across teams.** They assume an organization large enough that no single
> platform team can hand-hold every team's deploy.
>
> The platform team is treated as **leverage**, not **gatekeeper** — its
> product is the infrastructure that lets many teams ship without inventing
> many divergent deploy pipelines.

## How to read these rules

Each rule reads as a logical chain: a **business need** drives a
load-bearing **assumption**, which produces the **preference** that follows
from both.

- **Business need** — the organizational outcome the rule serves. When the
  outcome stops being a priority, revisit the rule.
- **Assumption** — what makes the chain sound. Falsify the assumption and
  the rule no longer applies.
- **Therefore prefer** — the decision shape that follows from the need +
  assumption.

Rules are heuristics, not laws. When a specific decision needs to violate
one, the violation belongs in the design discussion alongside the trade-off
that justifies it.

---

## 1. Bottom out at one file the team owns

**Business need.** Platform-team headcount and reviewer load grow
super-linearly when every team writes its own deploy pipeline. The
standard tooling has to stop where teams will actually maintain it, not
where it *could* go. Onboarding a new team should be a day, not a
quarter.

**Assumption.** Product teams let infrastructure they don't understand
rot. Asking them to maintain layers they don't use day-to-day is asking
for outages, security drift, and on-call escalations into the platform
team.

**Therefore prefer** designs that end at a single declarative deployment
file the team owns (typically a compose file). **Over** designs that
require teams to maintain Kubernetes manifests, Helm charts, or
service-mesh policy directly.

## 2. Catalogue over copy-paste

**Business need.** Centralized security, compliance, and audit posture;
the ability to ship cross-cutting infrastructure improvements once instead
of *N* times; fewer pages of the form "we found a bug in our copy of this
stack from three years ago."

**Assumption.** Without a shared source of truth, *N* teams converge on
*N* divergent forks of the same dependency. The platform team then loses
the ability to roll out cross-cutting changes (a CVE patch, a new audit
policy, a credentials rotation) without coordinating *N* migrations.

**Therefore prefer** published, versioned, centrally-maintained modules
that teams consume by reference. **Over** per-team copy-paste of the same
auth / observability / cache / data-tier stacks.

## 3. Prescription beats flexibility for composable things

**Business need.** Make team-to-team migrations possible; enable mass
updates; let new teams onboard against a known contract instead of the
union of every existing team's improvisations.

**Assumption.** Without prescription, every team renegotiates the
integration contract from scratch. Migrating between modules becomes
impossible because there's no shape to migrate *to*. Mass updates become
*N* separate conversations.

**Therefore prefer** a fixed module shape — interface contract,
configuration surface, versioning policy — defined by the catalogue.
**Over** "a module is whatever the team calls a module."

## 4. Defaults over knobs

**Business need.** Reduce misconfiguration incidents; shorten
time-to-value for new teams; keep audit-relevant configuration auditable
rather than buried under override layers; avoid the configuration sprawl
where each team's configuration file runs hundreds of lines because nothing
has a default.

**Assumption.** Default-setters (the platform team) have more context
about the organization-wide right answer than default-consumers (product
teams). A team should have to *override* a default deliberately, not
*choose* every value from scratch.

**Therefore prefer** one sensible default that the majority of teams
accept. **Over** an exhaustive configuration surface that no team
configures correctly.

## 5. Labels over generated output for review surface

**Business need.** Enable lightweight PR review by SREs and security
reviewers; make compliance-relevant choices auditable in the diff; avoid
"I didn't see that" incidents that follow even a successful audit.

**Assumption.** Reviewer attention is finite. What they read is what they
catch; what's behind a translation step they don't catch.

**Therefore prefer** surfacing important decisions where reviewers
already look — as labels or short declarative annotations alongside the
service definition. **Over** leaving important decisions in generated
chart output, container internals, or "if you grep deep enough you'll
find it."

## 6. Off-the-shelf for undifferentiated; custom for differentiated

**Business need.** Focus engineering hours on the org's differentiating
product; tap upstream maintenance, security patches, and feature work
without paying the headcount; reduce the build-vs-buy debt that
accumulates when build-by-default is the cultural norm.

**Assumption.** Every dependency you maintain is one you pay for later in
on-call hours, security patches, and feature-parity-with-the-OSS-thing
work. Building one of every layer doesn't deliver more value than
delivering the one layer that actually differentiates.

**Therefore prefer** mature open-source or vendor solutions for slots
where the team adds no value by building. **Over** building in-house every
layer the team will need.

## 7. Defer over-determined choices

**Business need.** Avoid early commitments that ossify the architecture
before real signals arrive; preserve optionality for the choices that
*will* matter — retention, tenancy, isolation, regional posture — once
the team has real volume, real SLAs, real query patterns, and real
compliance requirements to design against.

**Assumption.** Premature optimization compounds badly. Reversibility is
cheaper than precision — most "right calls" with insufficient information
become very wrong calls six months later, and the migration is much more
expensive than the initial decision was.

**Therefore prefer** a battle-tested default until production context is
known. **Over** making the "right" call with insufficient information.

---

## Conway's Law is the substrate, not the enemy

**Conway's Law** says any system reflects the communication structure of
the organization that built it. Most failed platform efforts try to
*fight* this — design a unified system that ignores team boundaries —
and get the predictable result: cross-team merge conflicts, integration
stalls, ownership ambiguity, and architecture that drifts back toward
team-shaped fault lines anyway.

The rules above are designed to **push with Conway's Law**, not against
it. Each one removes a place where the architecture could fight the org
structure:

- Each team owns one declarative deployment file (**Rule 1**) → the
  team becomes the deployment boundary, by design.
- Cross-cutting concerns live in shared, catalogued modules
  (**Rules 2 and 3**) → no team re-invents the same observability /
  auth / data-tier stack, and the platform team can roll out
  cross-cutting changes in one place.
- Defaults are set centrally (**Rule 4**) → platform owns the *shape*;
  product teams own the *product*. The seam runs where the headcount
  seam runs.
- Compliance-relevant choices surface alongside the service definition
  (**Rule 5**) → cross-team reviewers (SREs, security, audit) read
  along the team boundary, not around it.
- Off-the-shelf for undifferentiated work, custom for differentiated
  (**Rule 6**) → the org's engineering hours go where the org actually
  differentiates, not into rebuilding things every other org has
  already built.
- Defer over-determined choices (**Rule 7**) → the architecture stays
  responsive to whichever team owns the new context when it arrives,
  instead of being shaped by an early decision made before that team
  existed.

The architecture *is* the team structure. New team → new repo. Team
merger → repo merge. Team split → repo split. Org-chart changes are
deployment changes — a feature, not a bug.

This is why the seven rules form a coherent posture rather than seven
unrelated preferences. Each one closes a path where the platform could
have ended up fighting Conway's Law; what's left lines up with how the
work actually flows.

## When these rules don't apply

If your organization is small enough that the platform team can write
everyone's Helm chart by hand, the trade-offs invert: the cost of
prescription outweighs the cost of bespoke deploys, and bespoke is fine.
The rules above assume **scale** — where scale = "too many teams for one
team to hand-hold each."
