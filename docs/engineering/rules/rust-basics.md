# Top 10 Rust Best Practices

## 0. Data-source rule (highest priority)

**I will define the variables and files you are permitted to pull datapoints from. If I have not told you explicitly where to get the data, you must stop immediately.**

When data isn't where you expected (a missing field, an absent file, an empty struct), do not "fix" the gap by reaching for a different input — do not parse a sibling artifact, do not scrape a label meant for another consumer, do not invent a new ingestion path. Stop, surface the question, and wait for direction.

This rule is repeated in `CLAUDE.md`, `docs/engineering/rules/golang-basics.md`, `docs/engineering/rules/rust-basics.md`, `docs/engineering/rules/plugin-conventions.md`, `docs/engineering/rules/logging-platform.md`, and `docs/engineering/rules/katenary-top-seven.md`.

---

## 1. Ownership & Borrowing — Show Mastery
- Prefer borrowing (`&T`, `&mut T`) over cloning. Only `.clone()` when genuinely needed.
- Use `Cow<'_, str>` when you might or might not need ownership.
- Avoid `Rc`/`Arc` unless you have genuine shared ownership.

## 2. Error Handling — No `.unwrap()` in Production Code
- Define custom error types with `thiserror` (library) or `anyhow` (application).
- Use `?` propagation everywhere. `.unwrap()` only in tests or where invariants are proven.
- Implement `Display` and `Error` for your error types.

## 3. Use the Type System as a Guard Rail
- Newtype pattern (`struct UserId(u64)`) to prevent mixing up IDs, indices, etc.
- Enums for state machines — make illegal states unrepresentable.
- Parse, don't validate — convert raw input into typed structs at the boundary.

## 4. Traits & Generics — Idiomatic Abstraction
- Program against traits, not concrete types. `impl AsRef<str>` over `&String`.
- Use `impl Trait` in argument position for simple cases, named generics for complex ones.
- Derive `Debug`, `Clone`, `PartialEq` on your types — reviewers notice when it's missing.

## 5. Iterators Over Loops
- Prefer `.iter().map().filter().collect()` over manual `for` loops with `push`.
- Use `.enumerate()`, `.zip()`, `.chain()`, `.flat_map()` — show fluency.
- Avoid `.collect()`ing intermediate results when you can keep it lazy.

## 6. Lifetimes — Keep Them Simple
- Elision covers most cases. Only annotate when the compiler asks.
- If your structs have lifetimes getting complex, consider owning the data instead.
- Lifetime annotations document relationships — use them to clarify, not just to compile.

## 7. Concurrency — Fearless but Thoughtful
- Prefer channels (`mpsc`, `crossbeam`) over shared mutable state.
- `Arc<Mutex<T>>` when sharing is necessary — keep critical sections small.
- If async: use `tokio` idiomatically — `spawn`, `select!`, structured concurrency.

## 8. Project Structure & Module Organization
- `lib.rs` + `main.rs` split — keep logic testable, binary thin.
- Public API should be minimal — `pub(crate)` by default, `pub` only for the interface.
- One module per concept, re-export cleanly from `lib.rs`.

## 9. Testing — Show You Care About Correctness
- Unit tests in the same file (`#[cfg(test)] mod tests`).
- Integration tests in `tests/` for public API behavior.
- Use `proptest` or `quickcheck` if appropriate — instant senior signal.
- Test error cases, not just happy paths.

## 10. Performance Awareness — Don't Prematurely Optimize, But Don't Be Naive
- Prefer `&str` over `String` in function signatures.
- Use `Vec::with_capacity()` when size is known.
- Avoid unnecessary allocations — `format!` in hot loops, excessive `to_string()`.
- Know when to use `Box<dyn Trait>` vs generics (binary size vs monomorphization).

---

## Bonus Signals
- Good `Cargo.toml` hygiene (minimal deps, proper features)
- Meaningful commit messages
- Clippy clean code (`cargo clippy -- -D warnings`)
