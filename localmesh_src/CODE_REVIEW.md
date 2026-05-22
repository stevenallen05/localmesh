# Code review: localmesh CLI (Go)

Reviewed at commit `c87bd3d`. Date 2026-05-22. Scope is the Go source tree under
`localmesh_src/` only.

This tree is under active development. Commits landed during the review. Symbols
in the churning packages (`manifest`, `envwriter`, `render`) are referenced by
name rather than line number so this report survives the next few commits. The
stable `cmd/` references include line numbers.

## Verification run at review time

| Check | Command | Result |
|-------|---------|--------|
| Build | `go build ./...` | clean |
| Vet | `go vet ./...` | clean |
| Format | `gofmt -l .` | clean (no files listed) |
| Tests | `go test ./...` | all packages pass |

`golangci-lint` and `gofumpt` are configured in `.golangci.yml` but are not
installed in this environment, so the project's own lint gate was not exercised.
The `gofmt` baseline passed.

## Verdict

Strong conformance to Go CLI best practices. This is top-quartile work for a CLI
of this size. The structure, error handling, boundary validation, deterministic
output, and test coverage are all sound. The deviations are runtime ergonomics
and one testability gap. None are correctness or structural defects.

## Strengths

| Practice | Evidence |
|----------|----------|
| Thin `main`, testable core | `cmd/localmesh/main.go` is 31 lines. It wires subcommands and formats errors. All logic lives in `internal/` packages. |
| Idiomatic Cobra | `newXCmd() *cobra.Command` constructors. `RunE` so errors propagate. Flags bound locally (`ca.go:31`). Subcommand grouping under `ca`. `SilenceErrors` and `SilenceUsage` so Cobra does not double-print over `main`'s error format (`main.go:24`). |
| Error handling | Wrapped with `%w` plus context at every boundary. Sentinels `ErrMalformed` and `ErrAlreadyExists` support `errors.Is`. `CycleError` carries a readable `a -> b -> a` path. No bare `return err` without added context. |
| Parse, don't validate | Manifest loaders validate right after unmarshal and return typed structs. Scheme validation uses a closed-set map. Tri-state `*bool` with a `Meshed()` helper keeps "unset" distinct from "false". |
| Deterministic output | Sorted map keys in `envwriter.assemble` and `render.SortServices`. Dedup is first-wins. Re-renders are byte-identical, which is correct for generated, git-tracked artifacts. |
| Security hygiene | Mode `0600` on private keys and `0644` on certs. `crypto/rand` for serials and the OIDC secret. Mode choices are documented, including the ingress-secret rationale in `mtls.go`. |
| Avoided a real bug | `dexseed` uses `strings.NewReplacer` instead of `os.Expand` so bcrypt hashes in `dex.yaml.sample` are not corrupted. The reasoning is documented in the package comment. |
| Comments explain why | Load-bearing ordering notes in `setup.go`. Trade-off rationale and spec references throughout. TODOs use the project `needs_prod_decisions` convention. |
| Module hygiene | Minimal direct deps. Indirect deps segregated in `go.mod`. `.gitignore` covers the binary and test artifacts. One binary under `cmd/`. |

## Findings, ranked

### 1. No signal-cancellable context (the main CLI-idiom gap)

`main.go` calls `.Execute()`, not `.ExecuteContext(ctx)`. As a result
`cmd.Context()` returns `context.Background()` at every call site
(`setup.go:28`, `ca.go:28`, `ca.go:44`, `ca.go:58`). The `exec.CommandContext`
calls into `mkcert` are therefore not cancellable. `mkcert -install` can block on
a sudo prompt, so Ctrl-C and SIGTERM do not unwind it cleanly.

The context is already threaded through every command and into the `ca` package.
Only the root needs changing.

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    if err := newRootCmd().ExecuteContext(ctx); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

Low effort, high payoff.

### 2. `setup` orchestration is an untested closure

`build.go` and `ca.go` are thin delegates. `setup.go` is not. The full 7-step
bootstrap chain lives inside the `RunE` closure (`setup.go:23`-`114`). The
`cmd/localmesh` package has no test files, so that sequencing is unverified. This
includes the load-bearing order where `.env` is written before the compose
render.

Extract the body into an internal package function, for example
`setup.Run(ctx, repoRoot) error`. The `RunE` closure then becomes a thin
delegate like the other two commands, and the chain becomes unit-testable.

### 3. No `--version`

The root command has no `Version` field and there is no `-ldflags -X` version
injection. This is expected for a distributable CLI. Minor for a take-home.

### 4. Progress prints to stdout

`setup.go` prints `==> ...` progress lines via `fmt.Println`, which is stdout.
Convention is to send progress and diagnostics to stderr so stdout stays
machine-parseable. There is no machine-readable output today, so this is
defensible. Worth noting, not blocking.

## Not gaps

- Sequential `MintAll`. Clarity over speculative parallelism is the right call here.
- No `log/slog`. `fmt.Println` is fine at this scope for a bootstrap CLI.

## In-flight TOML work (not critiqued)

The `Deploy`, `Instance`, and `ValidateDeploy` machinery in `manifest` is landing
across the commits seen during this review. Per the request, the TOML schema
shape is not critiqued. One neutral observation, framed as transition state.
`LoadAll` currently returns `(project, plugins)` and does not yet load or apply
`deploy.toml`, even though `Instance` documents per-instance config and export
overrides. Wiring `LoadDeploy` and `ValidateDeploy` into the render path is
clearly the active work. This reads as in-progress, not as a defect.
