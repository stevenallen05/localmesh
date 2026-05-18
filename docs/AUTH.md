# Authentication — production architecture

If you've sat through a SOC 2 audit and asked, for the tenth time, why we have to keep renewing these certificates — this is the document that ends that.

A **service mesh** is one piece of infrastructure the ops team builds and runs, shipped to your stack as a single line you add to your compose file — the same shape as the observability and database modules you already include. You don't run it. You don't tune it. You include it. Three things land in your lap:

(a) **SOC 2 stops being personal.** The encrypted-traffic and service-identity controls live in the mesh policy itself. When an auditor asks "show me how internal traffic is protected," you point at one file and the ops team owns the explanation. You don't get put on the spot.

(b) **Certificates stop being your problem.** Every service gets a digital "ID card" issued automatically at startup and renewed on its own. No more `openssl` panic, no more "did this cert expire on a Saturday at 4am", no more cron job you wrote two years ago you're not sure still works.

(c) **A breach is contained for you, not by you.** The mesh denies all inter-service traffic by default. You list which services yours is allowed to call in a top-level compose flag. If something gets breached, the blast radius is whatever you wrote down — and security reviewers can see the answer at the top of your `docker-compose.yml`, the file you actually understand.

The same infrastructure also solves user authentication for every team at once — your team's new service doesn't have to build its own login handling, because the mesh already does it. And because every service is tagged with the team that owns it, when something goes wrong everyone knows who's on the hook: ops knows who to page, legal knows which team handled the data, and the exec team knows where to point the finger.

You enable the mesh by adding one line to your compose file — the same way you added the observability module. Ops supports the first onboarding; after that you copy-paste the pattern for the next service. The specific product the mesh runs on is ops's choice to make, not yours: your interface stays compose, the same way it does today.

Engineering detail: [`superpowers/specs/2026-05-15-identity-propagation-design.md`](./superpowers/specs/2026-05-15-identity-propagation-design.md) · Take-home scope: [`TODO.md`](./TODO.md).
