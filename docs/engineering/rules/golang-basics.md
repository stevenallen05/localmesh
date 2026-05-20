# Go basics — `localmesh` CLI

Authoritative for the Go code that lives under `localmesh_src/`. Scoped to what this specific CLI needs — sprig-templated compose synthesis, mkcert subprocess wrapping, native cert minting via `crypto/x509`, TOML + YAML I/O. Not a general Go style guide; for that read the standard Go review notes and `golangci-lint` defaults.

Read this first when adding code under `localmesh_src/`.

---

## 0. Data-source rule (highest priority)

**I will define the variables and files you are permitted to pull datapoints from. If I have not told you explicitly where to get the data, you must stop immediately.**

When data isn't where you expected (a missing field, an empty struct, an absent file), do not "fix" the gap by reaching for a different input — do not parse compose at build time, do not scrape labels meant for another consumer, do not introduce a new ingestion path to make the call site work. Stop, surface the question, and wait for direction.

This rule is repeated in `CLAUDE.md`, `docs/engineering/rules/plugin-conventions.md`, and `docs/engineering/rules/rust-basics.md` so it is re-encountered on every stack-specific lookup.

---

## 1. Module layout

```
localmesh_src/
├── go.mod                          # module: github.com/<team>/localmesh
├── go.sum
├── cmd/localmesh/main.go           # cobra root + verb wiring (thin)
├── internal/
│   ├── manifest/                   # plugin.toml + project.toml load/validate
│   ├── template/                   # sprig wrapper + custom helpers
│   ├── render/                     # compose merger; writes .localmesh/*.yaml
│   ├── ca/                         # root CA (mkcert wrapper)
│   ├── mtls/                       # leaf cert mint via crypto/x509
│   └── envwriter/                  # .env managed-section roundtrip
├── tools/                          # mkcert binary
└── testdata/sample_catalog/        # fixture plugins for tests
```

- **`cmd/` stays thin.** Cobra subcommand wiring + flag parsing. No business logic.
- **`internal/` holds the work.** Each package is independently testable; no `cmd/` imports inside `internal/`.
- **Package boundaries match data flow.** `manifest → template → render` is a one-way pipeline; `ca → mtls` is a separate vertical. No back-edges.
- **One concrete responsibility per package.** When a file grows past ~400 LOC, split by responsibility, not by technical layer.

---

## 2. Error handling

The project rule (matching `rust-basics.md`'s thiserror/anyhow convention): **sentinel errors at package boundaries; wrap with `fmt.Errorf("%w", err)` for context.**

```go
// Sentinel — packages that produce errors callers might branch on.
var ErrManifestMalformed = errors.New("plugin.toml malformed")

// Wrap with %w when propagating; preserves errors.Is / errors.As.
func Load(path string) (*Plugin, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read %s: %w", path, err)
    }
    var p Plugin
    if err := toml.Unmarshal(data, &p); err != nil {
        return nil, fmt.Errorf("parse %s: %w", path, ErrManifestMalformed)
    }
    return &p, nil
}
```

**MUST:**
- Wrap with `%w` on every propagation. Lets callers use `errors.Is` / `errors.As`.
- Include enough context that the error message is useful without a stack trace — file path, the operation, what was expected.
- Return errors. Never `log.Fatal` from `internal/`. The CLI entry point in `cmd/localmesh/main.go` is the only place that may exit; everything else returns.

**MUST NOT:**
- Panic outside of `init()` for genuine invariant violations. CLI users see panics as bugs, not validation failures.
- Use `_ = ...` to discard errors. If you don't care, comment why; if you might care, return it.
- Stringify errors mid-chain (`fmt.Errorf("...: %s", err.Error())`). That breaks `errors.Is`.

---

## 3. Context propagation

This CLI is short-lived and mostly synchronous. Context still belongs on every function that does I/O or could block:

```go
func (b *Builder) Render(ctx context.Context, plugins []Plugin) error { ... }
```

**MUST:**
- Pass `ctx context.Context` as the first parameter on any function that does file I/O, subprocess invocation, or template rendering (templates can hang on slow custom helpers).
- Propagate the root context down. Subcommands take `ctx := cmd.Context()` (cobra) and pass it through.
- Check `ctx.Err()` at clean checkpoint boundaries (between rendering each plugin, before each cert mint).

**MUST NOT:**
- Store context in structs. Pass it as an argument every time.
- Use `context.Background()` outside of `main()`. Always inherit.

---

## 4. CLI framework — cobra

Standard library `flag` is enough for trivial CLIs; cobra wins for nested verbs (`localmesh ca mint`, `localmesh ca install`).

**Pattern:**

```go
// cmd/localmesh/main.go
func main() {
    if err := newRootCmd().Execute(); err != nil {
        os.Exit(1)
    }
}

func newRootCmd() *cobra.Command {
    root := &cobra.Command{
        Use:           "localmesh",
        Short:         "LocalMesh dev-environment CLI",
        SilenceErrors: true, // we print our own error format
        SilenceUsage:  true,
    }
    root.AddCommand(newCACmd(), newMTLSCmd(), newBuildCmd())
    return root
}

func newBuildCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "build",
        Short: "Render .localmesh/localmesh.compose.yaml from plugin templates",
        RunE: func(cmd *cobra.Command, args []string) error {
            return render.Run(cmd.Context())
        },
    }
}
```

**Conventions:**
- `RunE` (returns error) not `Run` (swallows them).
- `SilenceErrors + SilenceUsage` on root; print errors via `fmt.Fprintln(os.Stderr, err)` in `main` so usage doesn't dump on every failure.
- Subcommand structure mirrors the verb tree: `ca mint`, `ca install`, `ca uninstall` are siblings of one `caCmd`.
- Flags on the leaf command, not on root, unless they're globally meaningful.

---

## 5. Library picks (locked for v0)

| Need | Library | Why |
|---|---|---|
| TOML | `github.com/BurntSushi/toml` | Most mature; clean struct mapping. |
| YAML | `gopkg.in/yaml.v3` | `yaml.Node` lets us control key ordering for deterministic output. |
| CLI framework | `github.com/spf13/cobra` | Standard for nested-verb CLIs in Go. |
| Template engine | `text/template` + `github.com/Masterminds/sprig/v3` | sprig is one-line drop-in on top of stdlib templates. |
| Logging | `log/slog` (stdlib, Go 1.21+) | Structured logging; no external dep. |
| Testing | stdlib `testing` | Plus `testdata/` for golden files. |

**`go.mod` hygiene:**
- Pin Go version (`go 1.22`).
- Minimum direct deps. Every transitive dep is a security surface; prefer stdlib where possible.
- `go mod tidy` before every commit.
- No replace directives unless explicitly justified (with a comment naming the upstream issue/PR).

---

## 6. Template engine — `text/template` + sprig

```go
import (
    "text/template"
    sprig "github.com/Masterminds/sprig/v3"
)

func New() *template.Template {
    t := template.New("plugin").
        Funcs(sprig.FuncMap()).
        Funcs(customFuncs()). // project-specific helpers
        Option("missingkey=error")
    return t
}

func customFuncs() template.FuncMap {
    return template.FuncMap{
        "spiffeURI":       spiffeURI,
        "identityLabels":  identityLabels,
    }
}
```

**MUST:**
- `Option("missingkey=error")` on every template. Default behavior is `<no value>` on missing keys — silent bugs in generated config.
- Strongly-typed context structs passed to `Execute`. Don't use `map[string]any` — readers can't see the schema.
- Project-specific helpers live in `internal/template/funcs.go`. Each helper is testable in isolation.

**MUST NOT:**
- Mutate the template context inside a helper. Helpers are pure functions.
- Use sprig functions that touch the host (`env`, `expandenv`) unless explicitly intended — those bypass the project's `.env` discipline.

---

## 7. Cert minting via `crypto/x509`

mkcert handles root CA + system trust install. Native Go owns leaf cert minting (mkcert can't emit URI SANs or per-cert lifetime).

```go
type LeafSpec struct {
    Container  string
    SPIFFEURI  *url.URL // spiffe://<container>.<project>.<domain>
    DNSNames   []string
    IPAddrs    []net.IP
    NotAfter   time.Time // 7d default; configurable
}

func (m *Minter) Mint(ctx context.Context, spec LeafSpec) error {
    // Load CA from .localmesh/secrets/root_ca/
    // Generate a new ECDSA key (or RSA; match mkcert default)
    // Build x509.Certificate template with SANs
    // x509.CreateCertificate signed by CA
    // Write id.crt, id.key, trust.ca.crt to .localmesh/secrets/<container>/
}
```

**MUST:**
- Validate inputs before signing. Empty container name, missing SANs, expired CA → return error.
- Use `os.WriteFile(path, data, 0600)` for `.key` files. CA loaders enforce strict mode.
- Use `pem.Encode` for PEM serialization; never hand-format.
- Deterministic file structure — fixed mode bits, fixed file names — so tests can compare bytes.

**MUST NOT:**
- Roll your own crypto. Stay in `crypto/x509` + `crypto/ecdsa` (or `crypto/rsa`); never `crypto/rand`-then-XOR-something.
- Trust user-provided SAN data. The container list comes from manifests, which are local; but always sanity-check.

---

## 8. Subprocess invocation — mkcert wrapper

```go
func (c *CA) Install(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, c.mkcertBin, "-install")
    cmd.Env = append(os.Environ(), "CAROOT="+c.caRoot)
    cmd.Stdout = os.Stdout // mkcert prompts; pass through
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("mkcert -install: %w", err)
    }
    return nil
}
```

**MUST:**
- Use `exec.CommandContext` (honors cancellation).
- Set `Env` explicitly when overriding (`CAROOT=...`). Don't mutate `os.Environ()`.
- Stream stdout/stderr for interactive prompts (mkcert install needs sudo prompt visibility).
- Pin the binary path (`localmesh_src/tools/mkcert`); don't trust `$PATH`.

**MUST NOT:**
- Construct shell strings (`exec.Command("sh", "-c", ...)`) for variable arguments. Use slice form to avoid injection.
- Ignore exit codes. `cmd.Run()` returns `*exec.ExitError` for non-zero; wrap and propagate.

---

## 9. Logging — `log/slog`

```go
import "log/slog"

logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
slog.SetDefault(logger)

slog.Info("rendered compose", "plugins", len(plugins), "out", ".localmesh/localmesh.compose.yaml")
slog.Warn("template helper used deprecated syntax", "plugin", name, "file", path)
```

**Conventions:**
- Use the package-level `slog.Info` / `slog.Error` (after `SetDefault`). No injected loggers needed for a single-binary CLI.
- Structured key-value pairs over format strings. `slog.Info("X", "key", value)`, not `slog.Info(fmt.Sprintf(...))`.
- `slog.Error` for failures that the CLI is about to exit on. `slog.Warn` for things that worked but the user should see.
- No `slog.Debug` in v0; chatty by default isn't helpful.

---

## 10. Testing

**Three layers:**

```go
// Unit (manifest_test.go)
func TestLoad(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr error
    }{
        {"happy path", `[identity]\nmodule_name = "x"\nowned_by = "x@x"`, nil},
        {"missing identity", `[[services]]\ncontainer = "x"`, ErrManifestMalformed},
        {"malformed toml", `not toml`, ErrManifestMalformed},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := LoadString(tt.input)
            if !errors.Is(err, tt.wantErr) {
                t.Fatalf("got %v, want %v", err, tt.wantErr)
            }
        })
    }
}

// Golden (render_test.go)
func TestRender(t *testing.T) {
    got, err := Render(testdata("sample_catalog"))
    if err != nil { t.Fatal(err) }
    golden := filepath.Join("testdata", "expected.compose.yaml")
    if *update {
        os.WriteFile(golden, got, 0644)
        return
    }
    want, _ := os.ReadFile(golden)
    if !bytes.Equal(got, want) {
        t.Fatalf("output drifted; UPDATE=1 go test to refresh")
    }
}

// Integration (//go:build integration)
//go:build integration
package render_test
func TestRealCatalog(t *testing.T) {
    cmd := exec.Command("docker", "compose", "-f", ".localmesh/localmesh.compose.yaml", "config")
    if err := cmd.Run(); err != nil { t.Fatal(err) }
}
```

**MUST:**
- Table-driven by default. Each row is a discrete scenario; `t.Run(tt.name, ...)` so individual cases can be filtered.
- Race detector on every CI run: `go test -race ./...`.
- Golden-file tests for the renderer. `UPDATE_GOLDEN=1 go test` is the regen path.
- Integration tests behind `//go:build integration` so the default unit loop stays fast.

**MUST NOT:**
- Test using time.Sleep. Use synchronization primitives or fake clocks.
- Skip race detector in CI. Even single-threaded code can hit data races via the testing framework.

---

## 11. Linting — `golangci-lint`

`localmesh_src/.golangci.yml`:

```yaml
linters:
  enable:
    - govet
    - errcheck
    - gosimple
    - staticcheck
    - unused
    - ineffassign
    - gofumpt
    - misspell
    - revive
  disable:
    - exhaustruct  # noisy for cobra command structs

linters-settings:
  errcheck:
    check-blank: true     # _ = ... must be explicit and justified

issues:
  exclude-rules:
    - path: _test\.go
      linters: [errcheck]  # test setup can be terser
```

**Run:**
- `golangci-lint run ./localmesh_src/...` (project-side)
- Pre-commit hook in `.pre-commit-config.yaml` (matches the existing Rust + JS lint hooks)

---

## 12. Configuration — functional options

For minters, builders, anything with multiple optional knobs:

```go
type Option func(*Minter)

func WithLifetime(d time.Duration) Option { return func(m *Minter) { m.lifetime = d } }
func WithECDSA() Option                   { return func(m *Minter) { m.useECDSA = true } }

func New(caRoot string, opts ...Option) *Minter {
    m := &Minter{caRoot: caRoot, lifetime: 7 * 24 * time.Hour}
    for _, opt := range opts { opt(m) }
    return m
}
```

**Why:** the API stays additive as knobs grow; callers don't have to pass `nil` for unused options.

**MUST NOT:** pass configuration via package-level globals. Tests can't run in parallel; library users can't override.

---

## 13. Quick reference — common pitfalls

- **`for _, x := range xs` capturing the loop variable.** Go 1.22 fixed this; if `go.mod`'s `go` line is `1.21` or earlier, capture explicitly: `x := x; go func() { use(x) }()`.
- **Map iteration is randomized.** When YAML output must be deterministic, sort keys explicitly.
- **`defer` in a loop accumulates.** If you're opening files in a loop, wrap the body in a function so `defer` fires per iteration.
- **`time.Now()` in tests.** Inject a clock function or stub via build tags. Hardcoded time comparisons are flaky.
- **`os.ReadFile` returns `[]byte` not `string`.** Cast explicitly; sprig helpers expect `string`.
- **Forgetting `cmd.Wait()` after `cmd.Start()`.** Leaks the child process. Use `cmd.Run()` unless you need streaming.

---

## 14. What this doc deliberately doesn't cover

- gRPC patterns (the CLI doesn't speak network protocols)
- HTTP server / handler design (no server in v0)
- Concurrent goroutine choreography (one synchronous pipeline; no fanout)
- Cgo / FFI (mkcert is a subprocess, not linked)
- Reflection patterns (avoid in this codebase)

If a future feature needs one of these, this doc grows or splits.
