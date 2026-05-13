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

Each rule has three parts:

- **Prefer / over** — the decision shape.
- **Assumption** — what makes the rule sound. Falsify the assumption and
  the rule no longer applies.
- **Business need** — the organizational outcome the rule serves. When the
  outcome stops being a priority, revisit the rule.

Rules are heuristics, not laws. When a specific decision needs to violate
one, the violation belongs in the design discussion alongside the trade-off
that justifies it.

---

## 1. Bottom out at one file the team owns

**Prefer** designs that end at a single declarative deployment file the
team owns (typically a compose file).
**Over** designs that require teams to maintain Kubernetes manifests, Helm
charts, or service-mesh policy directly.

**Assumption.** Product teams let infrastructure they don't understand
rot. Asking them to maintain layers they don't use day-to-day is asking
for outages, security drift, and on-call escalations into the platform
team.

**Business need.** Platform-team headcount and reviewer load grow
super-linearly when every team writes its own deploy pipeline. The
standard tooling has to stop where teams will actually maintain it, not
where it *could* go. Onboarding a new team should be a day, not a
quarter.

## 2. Catalogue over copy-paste

**Prefer** published, versioned, centrally-maintained modules that teams
consume by reference.
**Over** per-team copy-paste of the same auth / observability / cache /
data-tier stacks.

**Assumption.** Without a shared source of truth, *N* teams converge on
*N* divergent forks of the same dependency. The platform team then loses
the ability to roll out cross-cutting changes (a CVE patch, a new audit
policy, a credentials rotation) without coordinating *N* migrations.

**Business need.** Centralized security, compliance, and audit posture;
the ability to ship cross-cutting infrastructure improvements once instead
of *N* times; fewer pages of the form "we found a bug in our copy of this
stack from three years ago."

## 3. Prescription beats flexibility for composable things

**Prefer** a fixed module shape — interface contract, configuration
surface, versioning policy — defined by the catalogue.
**Over** "a module is whatever the team calls a module."

**Assumption.** Without prescription, every team renegotiates the
integration contract from scratch. Migrating between modules becomes
impossible because there's no shape to migrate *to*. Mass updates become
*N* separate conversations.

**Business need.** Make team-to-team migrations possible; enable mass
updates; let new teams onboard against a known contract instead of the
union of every existing team's improvisations.

## 4. Defaults over knobs

**Prefer** one sensible default that the majority of teams accept.
**Over** an exhaustive configuration surface that no team configures
correctly.

**Assumption.** Default-setters (the platform team) have more context
about the organization-wide right answer than default-consumers (product
teams). A team should have to *override* a default deliberately, not
*choose* every value from scratch.

**Business need.** Reduce misconfiguration incidents; shorten
time-to-value for new teams; keep audit-relevant configuration auditable
rather than buried under override layers; avoid the configuration sprawl
where each team's `values.yaml` runs hundreds of lines because nothing has
a default.

## 5. Labels over generated output for review surface

**Prefer** surfacing important decisions where reviewers already look — as
labels or short declarative annotations alongside the service definition.
**Over** leaving important decisions in generated chart output, container
internals, or "if you grep deep enough you'll find it."

**Assumption.** Reviewer attention is finite. What they read is what they
catch; what's behind a translation step they don't catch.

**Business need.** Enable lightweight PR review by SREs and security
reviewers; make compliance-relevant choices auditable in the diff; avoid
"I didn't see that" incidents that follow even a successful audit.

## 6. Off-the-shelf for undifferentiated; custom for differentiated

**Prefer** mature open-source or vendor solutions for slots where the team
adds no value by building.
**Over** building in-house every layer the team will need.

**Assumption.** Every dependency you maintain is one you pay for later in
on-call hours, security patches, and feature-parity-with-the-OSS-thing
work. Building one of every layer doesn't deliver more value than
delivering the one layer that actually differentiates.

**Business need.** Focus engineering hours on the org's differentiating
product; tap upstream maintenance, security patches, and feature work
without paying the headcount; reduce the build-vs-buy debt that
accumulates when build-by-default is the cultural norm.

## 7. Defer over-determined choices

**Prefer** a battle-tested default until production context is known.
**Over** making the "right" call with insufficient information.

**Assumption.** Premature optimization compounds badly. Reversibility is
cheaper than precision — most "right calls" with insufficient information
become very wrong calls six months later, and the migration is much more
expensive than the initial decision was.

**Business need.** Avoid early commitments that ossify the architecture
before real signals arrive; preserve optionality for the choices that
*will* matter — retention, tenancy, isolation, regional posture — once
the team has real volume, real SLAs, real query patterns, and real
compliance requirements to design against.

---

## Applying these rules

The rules form a coherent posture: a platform that **doesn't ask product
teams to become deploy experts**, that **scales its decisions
horizontally** across teams, and that **defers irreversible commitments
until the data arrives**.

If your organization is small enough that the platform team can write
everyone's Helm chart by hand, the trade-offs invert: the cost of
prescription outweighs the cost of bespoke deploys, and bespoke is fine.
The rules above assume **scale** — where scale = "too many teams for one
team to hand-hold each."
