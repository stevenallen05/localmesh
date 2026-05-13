# Engineering rules

Design rules behind this architecture. Two goals: **velocity of product-team shipping** and **architectural coherence across teams.** Assumes scale — too many teams for one platform team to hand-hold each. The platform team is leverage, not gatekeeper.

Rules are heuristics, not laws. Violating one means showing the trade-off.

## 1. Bottom out at one file the team owns

Designs end at a single declarative deployment file (typically a compose file) the team can maintain — not Kubernetes manifests, Helm charts, or mesh policy directly.

*Why:* Reviewer load grows super-linearly when every team writes its own pipeline; product teams let infrastructure they don't understand rot.

## 2. Catalogue over copy-paste

Cross-cutting concerns (auth, observability, cache, data tier) live in published, versioned, centrally-maintained modules consumed by reference.

*Why:* Otherwise *N* teams converge on *N* divergent forks, and the platform team can't ship a CVE (Common Vulnerabilities and Exposures) patch in one place.

## 3. Prescription beats flexibility for composable things

A module's interface contract, configuration surface, and versioning policy are fixed by the catalogue, not renegotiated per team.

*Why:* Without prescription there's no shape to migrate *to*; mass updates fragment into *N* conversations.

## 4. Defaults over knobs

One sensible default the majority of teams accept, not an exhaustive config surface no team configures correctly.

*Why:* Default-setters (platform team) have more org-wide context than default-consumers (product teams); overrides should be deliberate, not the starting point.

## 5. Labels over generated output for review surface

Compliance-relevant decisions surface as labels alongside the service definition — where SREs (Site Reliability Engineers) and security reviewers already look, not buried in generated chart output or container internals.

*Why:* Reviewer attention is finite; what's behind a translation step doesn't get caught.

## 6. Off-the-shelf for undifferentiated; custom for differentiated

Mature OSS (open-source software) or vendor for slots where the team adds no value by building.

*Why:* Every dependency you maintain is one you pay for in on-call, security patches, and feature-parity work. Engineering hours go where the org actually differentiates.

## 7. Defer over-determined choices

Battle-tested default until production context is known.

*Why:* Reversibility is cheaper than precision — "right calls" with insufficient information become very wrong calls six months later, and the migration costs more than the initial decision did.

---

## Conway's Law as substrate

Conway's Law: systems mirror the communication structure of the organizations that build them. The rules above push **with** that, not against it:

- One deployment file per team (Rule 1) → team = deployment boundary.
- Cross-cutting concerns in shared modules (Rules 2–3) → platform owns the *shape*, product owns the *product*.
- Centrally-set defaults (Rule 4) → the seam runs where the headcount seam runs.
- Compliance choices in the diff (Rule 5) → cross-team reviewers read along team lines.
- Off-the-shelf for undifferentiated (Rule 6) → engineering hours go where the org differentiates.
- Deferred over-determined choices (Rule 7) → architecture stays responsive to whichever team owns the new context.

The architecture *is* the team structure. New team → new repo. Team merge → repo merge. Org-chart changes are deployment changes.

## When these rules don't apply

If the platform team can hand-write everyone's chart, the trade-offs invert and bespoke is fine. These rules assume scale.
