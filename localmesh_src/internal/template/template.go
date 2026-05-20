// Package template renders plugin docker-compose.yaml.gotmpl files
// against project + plugin context using text/template + sprig.
//
// See docs/engineering/rules/golang-basics.md §6 for style.
package template

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

// WorkloadCtx is a per-workload entry surfaced to plugin templates that
// need to iterate non-exempt workloads (e.g. mesh's docker-compose.yaml.gotmpl
// emits one sidecar block per workload). Populated by render.RunWith after
// it has merged enough of the compose surface to know every service's
// name + mesh.exempt label.
type WorkloadCtx struct {
	Name       string
	MeshExempt bool
}

// Context is the data exposed to every template.
type Context struct {
	Project   *manifest.Project
	Plugin    *manifest.Plugin
	Env       map[string]string
	Workloads []WorkloadCtx // post-merge view; nil on the first render pass
}

// Render reads a .gotmpl file and returns its rendered output.
// Plain .yaml files (no .gotmpl suffix) are returned as-is.
func Render(path string, ctx *Context) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if !strings.HasSuffix(path, ".gotmpl") {
		return raw, nil
	}
	tmpl, err := template.New(path).
		Funcs(sprig.FuncMap()).
		Funcs(customFuncs()).
		Option("missingkey=error").
		Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, ctx); err != nil {
		return nil, fmt.Errorf("render %s: %w", path, err)
	}
	return out.Bytes(), nil
}
