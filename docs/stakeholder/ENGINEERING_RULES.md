# Engineering rules

Design rules behind this architecture. Two goals: product teams shipping fast, and the architecture staying coherent across many teams.

Assumes scale — too many teams for one platform team to hand-hold. The platform team builds infrastructure that makes other teams faster, not a gate they have to pass through.

Heuristics, not laws. Breaking one means showing the trade-off.

## 1. Each team owns one deployment file

A team's deploy story ends at one declarative file they actually read and edit — typically a compose file. Not Kubernetes manifests, not hand-edited Helm charts, not mesh policy. The platform team's tooling stops there.

*Why:* Teams let infrastructure they don't understand go stale. That turns into outages, security gaps, and pages to the platform team.

## 2. Cross-cutting concerns live in a shared catalogue

Things every service needs — auth, observability, cache, data tier — come from a centrally-maintained set of modules called the *catalogue*. Published, versioned, consumed by reference. Teams don't copy-paste their own.

*Why:* Otherwise you get many slightly-different copies of the same thing. A security fix becomes N migrations instead of one update.

## 3. The catalogue prescribes; teams don't renegotiate

Each module has a fixed shape: defined interface, defined configuration, defined versioning policy. Teams take it on those terms or pick a different module.

*Why:* If every team can change the module's contract, there's no clean target to migrate everyone *to*. Big updates — a new auth flow, a new metrics format — fragment into many conversations instead of one.

## 4. Sensible defaults beat exhaustive configuration

Pick one configuration most teams accept. Allow overrides for special cases, but the override should be deliberate and visible — not the starting point.

*Why:* The platform team knows the org-wide right answer better than the team using the module. Without strong defaults, configs grow to hundreds of lines and nobody gets it right.

## 5. Important decisions go where reviewers already look

If a choice matters for security, compliance, or audit, surface it as a **top-level** `x-katenary:` flag in the team's `docker-compose.yml` — where SREs (Site Reliability Engineers) and security reviewers already read, *and* where one declaration covers every service in the stack. The chart generator (or a future linter) derives the per-service rules from that flag: log retention, encryption at rest, audit trail, backup policy. Not buried in generated chart output, not scattered as per-service labels, not in container internals.

*Why:* Reviewer attention is finite. What's behind a translation step doesn't get caught, even by good reviewers.

## 6. Use off-the-shelf for undifferentiated work

If a layer isn't where the team adds product value — auth, metrics storage, dashboards, secret rotation — use a mature open-source or vendor option. Build custom only for the parts that differentiate the org.

*Why:* Every layer you build is one you pay for in on-call, security patches, and keeping up with whatever the open-source equivalent does next. Engineering hours go where the org wins.

## 7. Defer choices until production tells you the answer

When a decision depends on facts you don't have yet (real traffic, availability targets, compliance requirements, customer mix), use the battle-tested default and revisit when production arrives.

*Why:* "Right calls" with too little information become very wrong calls six months later, and the migration costs more than the original decision did. Reversibility is cheaper than precision.

---

## Conway's Law

Conway's Law: any system mirrors the communication structure of the organization that built it. Teams that don't talk produce code that doesn't interface cleanly; responsibilities split among people show up split in the architecture, whether you intended that or not.

This architecture leans into that instead of fighting it. Technical boundaries sit where team boundaries already are. Each team gets one repo and one deployment file. Cross-cutting concerns no single team owns — auth, observability, the data tier — live in a shared catalogue the platform team maintains as its own product. Compliance choices show up at the top of the team's `docker-compose.yml` as `x-katenary:` flags — one declaration per stack, visible to cross-team reviewers (SREs, security, audit) without grepping each service.

The architecture *is* the team structure. New team → new repo. Two teams merge → their repos merge. Org-chart changes are deployment changes — by design.

## When these rules don't apply

These assume scale — too many teams for one platform team to hand-hold. If the platform team can hand-write every team's chart, the trade-offs invert and bespoke is fine.
