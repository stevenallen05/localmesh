// Package enduserattrs provides a Caddy v2 HTTP handler that copies
// configured request headers onto the active OTel span as attributes.
// Used by LocalMesh's ingress to stamp enduser.{id,email} from
// X-Forwarded-{User,Email} for the PII-at-ingress rule.
package enduserattrs

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	caddy.RegisterModule(Handler{})
	httpcaddyfile.RegisterHandlerDirective("enduser_attrs", parseCaddyfile)
}

// Handler copies request headers onto the active OTel span as
// attributes. Configured as a list of "<header>=<attr>" pairs.
type Handler struct {
	Mapping map[string]string `json:"mapping,omitempty"`
}

func (Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.enduser_attrs",
		New: func() caddy.Module { return new(Handler) },
	}
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	span := trace.SpanFromContext(r.Context())
	if span.IsRecording() {
		for header, attr := range h.Mapping {
			if v := r.Header.Get(header); v != "" {
				span.SetAttributes(attribute.String(attr, v))
			}
		}
	}
	return next.ServeHTTP(w, r)
}

// Caddyfile syntax:
//   enduser_attrs {
//       X-Forwarded-User enduser.id
//       X-Forwarded-Email enduser.email
//   }
func (h *Handler) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	h.Mapping = make(map[string]string)
	for d.Next() {
		for d.NextBlock(0) {
			header := d.Val()
			if !d.NextArg() {
				return d.ArgErr()
			}
			h.Mapping[header] = d.Val()
		}
	}
	return nil
}

func parseCaddyfile(helper httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	h := new(Handler)
	if err := h.UnmarshalCaddyfile(helper.Dispenser); err != nil {
		return nil, err
	}
	return h, nil
}

var (
	_ caddy.Module                = (*Handler)(nil)
	_ caddyhttp.MiddlewareHandler = (*Handler)(nil)
	_ caddyfile.Unmarshaler       = (*Handler)(nil)
)
