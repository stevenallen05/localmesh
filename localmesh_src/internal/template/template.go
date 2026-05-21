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

// ServiceCtx is a per-service entry surfaced to plugin templates. The security
// plugin's compose template iterates it: a kuma-dp sidecar per Meshed service,
// a MeshHTTPRoute per ExposeViaIngress service. Populated by render.RunWith
// from the loaded manifests (project + plugin [[services]]).
type ServiceCtx struct {
	Name             string
	Port             int
	Meshed           bool
	ExposeViaIngress bool
}

// Context is the data exposed to every template.
type Context struct {
	Project  *manifest.Project
	Plugin   *manifest.Plugin
	Env      map[string]string
	Services []ServiceCtx // all registry-known services; plugins that don't need it ignore it
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
