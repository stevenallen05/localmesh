# CLAUDE.md

## Data-source rule (read every session, every rule file)

**I will define the variables and files you are permitted to pull datapoints from. If I have not told you explicitly where to get the data, you must stop immediately.**

You may not "work around" missing data by inappropriately introducing tight coupling between layers — e.g. reaching into a sibling file, parsing a runtime artifact at build time, or scraping labels meant for another consumer. When the data isn't where you expected, the correct move is to stop and ask, not to invent a new ingestion path.

This rule is repeated in `docs/engineering/rules/golang-basics.md`, `docs/engineering/rules/plugin-conventions.md`, and `docs/engineering/rules/rust-basics.md` so you re-encounter it on every stack-specific lookup. If a rule file is missing it, restore it.

NOTE: ALL COMMITS MUST HAVE STEALTH-MODE COMMIT MESSAGES. NO EXCEPTIONS. Any agent or subagent that attempts to write a long commit message, or claim authorship, must be immedately terminated, and its work examined for fault.

When doing Golang work, gopls _must_ be used. If gopls can't be found or doesn't work, that MUST be fixed before doing any further work.

Any git failures mean you must pause what you're doing and ask for guidance.

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project context

This is a **take-home assignment**, not production. Optimize for shortest path to something working end-to-end; depth comes from `docs/DESIGN_DECISIONS.md`, not from extra layers in the code. When a real-world choice would differ from the demo-scoped one, say so inline — short form, e.g. *"after production validation, swap the in-memory queue for Redis"* — instead of building the production version now. Code snippets in docs and comments should omit boilerplate (imports, error wiring, ceremony) and show only the load-bearing lines.

## Engineering record

- `docs/DESIGN_DECISIONS.md` is the **canonical** running record of engineering decisions. Read it before any non-trivial change; update it in the same change that lands the decision.
- It reflects the **current** shape of each choice, not its history. When a decision changes, edit the entry in place — do not append "superseded by…" notes, do not version it, do not start a parallel `docs/adr/` tree.
- **Git is the history.** Use `git log -- docs/DESIGN_DECISIONS.md` (or `git blame`) to see prior states. Do not write changelogs into the doc itself, and do not create CHANGELOG.md, decision logs, or audit trails alongside it.
- Decisions still in flux belong in an "Open decisions" section of the doc; move them out once they settle.
- Don't add new top-level docs (ARCHITECTURE.md, NOTES.md, etc.) — fold the content into `DESIGN_DECISIONS.md` or into the relevant code's doc comments. Ask before adding files under `docs/`.

## Stack-specific expectations

If `docs/rust-basics.md` or other stack-specific style notes exist in this repo, treat them as authoritative for that stack and follow them over the generic guidance below.

### Rust

- `lib.rs` + `main.rs` split so logic is testable and the binary stays thin. Default visibility `pub(crate)`; `pub` only for the deliberate interface.
- Errors: `thiserror` for libraries, `anyhow` for binaries. Propagate with `?`. No `.unwrap()`/`.expect()` outside tests or proven invariants — and when used, the invariant goes in the message.
- Borrow over clone; `Cow<'_, str>` when ownership is conditional. `Arc`/`Rc` only for genuine shared ownership, not as a borrow-checker escape hatch.
- Newtypes for IDs/indices; enums for state. Make illegal states unrepresentable. Parse at boundaries, don't validate after the fact.
- Prefer `&str`/`AsRef<str>`/`impl Trait` in arg position; named generics when bounds get reused. Derive `Debug`, `Clone`, `PartialEq` where it costs nothing.
- Iterators over manual loops; keep chains lazy — don't `.collect()` intermediate results.
- Concurrency: prefer channels and structured concurrency. `Arc<Mutex<_>>` is fine but keep critical sections tiny. With Tokio, watch for blocking calls inside async — use `spawn_blocking`.
- Keep it clippy-clean: `cargo clippy -- -D warnings`. Format with `cargo fmt`.
- `Cargo.toml` hygiene: minimal default features, explicit feature flags on heavy deps, no unused entries.

### Go

- Errors are values. Wrap with `%w` (`fmt.Errorf("doing X: %w", err)`); compare with `errors.Is`/`errors.As`. Don't return bare `err` without context unless the caller is the one adding it.
- Accept interfaces, return concrete types. Define interfaces on the consumer side, not the producer side.
- Contexts thread through every call that does I/O or could block — `ctx context.Context` is the first parameter. Don't store contexts in structs.
- Goroutines need an owner and a shutdown path. Use `errgroup.WithContext` or explicit `sync.WaitGroup` + cancellation; never spawn a goroutine you can't stop.
- `defer` for cleanup; check the `Close()` error if it can fail (especially writers). Beware deferred `Close` in a loop — wrap the body in a function.
- Slices and maps are reference-like; copy when escaping the current scope if the caller might mutate. `make([]T, 0, n)` when the size is known.
- Tests: table-driven by default. Subtests with `t.Run(name, …)` so individual cases can be filtered. `t.Helper()` in assertion utilities.
- Module hygiene: one binary per `cmd/<name>`. Run `go vet ./...` and `gofmt`/`goimports`. Add `staticcheck` if available.

### Next.js

- Know which router you're in (App Router under `app/` vs Pages Router under `pages/`) — the rules and conventions don't cross. Don't mix idioms; don't migrate piecemeal without a plan.
- App Router: components are **server components by default**. `"use client"` is opt-in and viral down the tree. Push client boundaries as far toward the leaves as possible — interactive bits, not whole pages.
- Don't import server-only modules (DB clients, secrets, `fs`, gRPC clients) into client components. Use `server-only` / `client-only` packages to enforce the boundary at build time.
- Data fetching: prefer server components and Route Handlers / Server Actions over client-side fetches. Cache and revalidation are explicit (`fetch` options, `revalidatePath`, `revalidateTag`) — don't rely on defaults silently.
- API routes / Route Handlers are serverless-shaped. Don't keep long-lived state, in-memory caches, or open sockets in them across requests — they will not survive cold starts or instance scale-out.
- Environment variables: only `NEXT_PUBLIC_*` reach the browser. Anything sensitive must not have that prefix and must not be referenced from client components.
- TypeScript: keep `strict: true` aspirationally. When `strict` is off, treat `any` and implicit `any` as smells worth flagging.
- Build hygiene: `npm run lint` and `npm run build` should pass; a successful `dev` is not proof of a successful `build`.

### Docker & containerization

- Multi-stage builds: heavy build image, slim runtime image. The runtime stage should contain only the binary and what it needs — not the toolchain.
- Order `COPY` and `RUN` instructions from least-to-most volatile so the cache survives day-to-day edits. Copy dep manifests (`go.mod`/`go.sum`, `Cargo.toml`/`Cargo.lock`, `package*.json`) and run dep fetch *before* copying source.
- Pin base images by digest or a specific minor tag (`debian:bookworm-slim`, `node:22-alpine`) — never `:latest`. Bump pins deliberately.
- Run as a non-root user. Add a `USER` directive. Drop unnecessary capabilities at run time (`--cap-drop=ALL` plus what's needed).
- `EXPOSE` is documentation, not enforcement. The actual port mapping happens at `docker run` / compose. Don't hardcode host ports inside the image.
- PID 1 matters: signals must reach your process for graceful shutdown. Use `ENTRYPOINT ["exec-form"]`, not shell-form; consider `tini` or `dumb-init` when the runtime doesn't reap zombies.
- Compose: name services by role (`server`, `agent`, `db`). Use `depends_on` with `condition: service_healthy` and define real `healthcheck`s — `depends_on` without conditions only orders startup, not readiness.
- Don't bake secrets into images or commit `.env` files with real values. Use compose `secrets`, mount files, or inject at run time.
- `.dockerignore` is as important as `.gitignore` — exclude `target/`, `node_modules/`, `.git/`, build caches. A bloated build context silently wrecks build speed.

### gRPC / Protocol Buffers

- Protos are an API contract — treat changes the way you'd treat a public HTTP API. Renaming a field is a breaking change even if the tag stays the same (clients deserialize by name in some languages).
- Field numbers are forever. Never reuse a removed field's number; mark it `reserved`. Don't change a field's type. Adding optional fields is safe; making a singular field `repeated` is not.
- Prefer `oneof` over a pile of optional fields for "exactly one of these" semantics. Reserve a `oneof` slot for future variants when you can predict them.
- Return meaningful gRPC `Status` codes — `INVALID_ARGUMENT`, `NOT_FOUND`, `FAILED_PRECONDITION`, `UNAVAILABLE`, `DEADLINE_EXCEEDED` carry information. `UNKNOWN`/`INTERNAL` for everything is an anti-pattern.
- Streams: every stream needs a defined termination contract (who closes, when, on what error). Long-lived bidi streams need keepalive tuned on both client and server, plus a backoff/reconnect strategy.
- Deadlines flow from caller to callee — set them on the client; respect `ctx.Done()` / cancellation on the server. Avoid unbounded server-side work for a request whose client gave up.
- Generated code is build output. Either commit it consistently (with the generator pinned) or generate it during the build — don't do both, and don't let the two drift.

## Working style

- Small, focused changes; lean on the type checker / clippy / `go vet` / `tsc --noEmit` instead of speculative refactors.
- Don't introduce abstraction layers, generics, or interfaces for a single concrete caller. Wait for the second use case.
- Don't add backwards-compatibility shims, dead-code paths, or feature flags for hypothetical futures — git can resurrect anything that's actually needed.
- When in doubt about a decision the project has already made, `docs/DESIGN_DECISIONS.md` first, then `git log`. Don't re-derive choices from the code.
