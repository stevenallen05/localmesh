#!/usr/bin/env python3
"""LocalMesh secrets bootstrap.

Reads `project.toml` (repo root) and every `service_catalog/*/plugin.toml`,
then:

  1. Mints the LocalMesh CA at `.secrets/certs/ca.{crt,key}` (one-shot).
  2. Mints a bidirectional leaf cert per declared service at
     `.secrets/certs/<container>/{trust.ca.crt, id.crt, id.key}` — skipped
     for services carrying `mesh.exempt: "true"` on their compose labels.
  3. Writes `.env` with UPCASE_SNAKECASE versions of every TOML key (in a
     managed section between `# >>> secrets-gen managed` markers).
  4. Generates `service_catalog/caddy/Caddyfile.generated` with one site
     block per service that has `expose_via_ingress = true` — mTLS upstream
     for mesh-participating services, plaintext for mesh-exempt ones.

Mesh-exempt status is the compose label `mesh.exempt: "true"` declared
in each plugin's `docker-compose.yml`, not a TOML field. This script
parses each plugin's compose to derive the exempt set.

See docs/superpowers/specs/2026-05-18-localmesh-service-mesh-design.md.

TODO: needs_prod_decisions IP SAN list is over-permissive for dev convenience.
TODO: needs_prod_decisions SPIFFE URI shape — flat (`spiffe://<svc>.<proj>.<dom>`)
       is dev-only; prod uses canonical `spiffe://<trust-domain>/<workload-path>`.
TODO: needs_prod_decisions cert lifespan — 10y dev; prod uses SVID rotation.
TODO: needs_prod_decisions managed-section `.env` is the bridge — proper
       compose extension or build-step overlay replaces it later.
"""
from __future__ import annotations

import os
import pathlib
import secrets as _secrets
import subprocess
import sys
import tomllib

import yaml

REPO = pathlib.Path(os.environ.get("REPO_ROOT", ".")).resolve()
PROJECT_TOML = REPO / "project.toml"
CATALOG = REPO / "service_catalog"
SECRETS = REPO / ".secrets"
CERTS = SECRETS / "certs"
ENV_FILE = REPO / ".env"
CADDYFILE = CATALOG / "caddy" / "Caddyfile.generated"
STEP = str(REPO / "tools" / "step")

# Over-permissive for dev. See TODO above.
IP_SANS = [
    "127.0.0.1", "::1", "0.0.0.0",
    "10.0.0.1", "172.17.0.1", "172.18.0.1", "172.19.0.1", "172.20.0.1",
    "192.168.65.1", "192.168.1.1",
]

MANAGED_START = "# >>> secrets-gen managed — do not edit; regenerated on `make certs`"
MANAGED_END = "# <<< secrets-gen managed"


# ---------------------------------------------------------------------------
# Manifest loading

def load_manifests() -> tuple[dict, list[tuple[str, dict]]]:
    """Return (project, [(plugin_slug, plugin_dict), ...])."""
    project = tomllib.loads(PROJECT_TOML.read_text())
    plugins: list[tuple[str, dict]] = []
    for manifest in sorted(CATALOG.rglob("plugin.toml")):
        plugins.append((manifest.parent.name, tomllib.loads(manifest.read_text())))
    return project, plugins


def compose_exempts(plugin_slug: str) -> set[str]:
    """Containers in plugin's docker-compose.yml carrying `mesh.exempt: "true"`."""
    compose = CATALOG / plugin_slug / "docker-compose.yml"
    if not compose.exists():
        return set()
    doc = yaml.safe_load(compose.read_text()) or {}
    exempt: set[str] = set()
    for svc_name, svc in (doc.get("services") or {}).items():
        labels = (svc or {}).get("labels") or {}
        # labels can also be a list of "key=value" strings; this project uses dicts.
        if isinstance(labels, dict) and str(labels.get("mesh.exempt", "")).lower() == "true":
            container_name = (svc or {}).get("container_name", svc_name)
            exempt.add(container_name)
    return exempt


def services_iter(project: dict, plugins: list[tuple[str, dict]]):
    """Yield (container, port, expose_via_ingress, mesh_exempt, ingress, owner_slug, requires_auth).

    mesh_exempt is derived from each plugin's docker-compose.yml labels
    (`mesh.exempt: "true"`), not from plugin.toml. Project-level services
    are never mesh-exempt (the app tier always joins the mesh).

    requires_auth defaults to True (default-deny at the ingress). Services
    that explicitly opt out — the IdP itself, primarily — set
    `requires_auth = false` in their `[[services]]` entry.
    """
    for svc in project.get("services", []):
        yield (
            svc["container"],
            svc["port"],
            svc.get("expose_via_ingress", False),
            False,
            svc.get("ingress", False),
            "project",
            svc.get("requires_auth", True),
        )
    for slug, plugin in plugins:
        exempt_set = compose_exempts(slug)
        for svc in plugin.get("services", []):
            yield (
                svc["container"],
                svc["port"],
                svc.get("expose_via_ingress", False),
                svc["container"] in exempt_set,
                svc.get("ingress", False),
                slug,
                svc.get("requires_auth", True),
            )


# ---------------------------------------------------------------------------
# Cert generation

def san_list_for(container: str, project_name: str, local_domain: str, is_ingress: bool) -> list[str]:
    """Bare SAN values — step CLI autodetects type from value shape.

    URIs are detected by the presence of `://`; IPs by valid IP parse;
    everything else is treated as a DNS name. So we pass values raw,
    without `URI:`/`DNS:`/`IP:` prefixes.
    """
    sans = [
        f"spiffe://{container}.{project_name}.{local_domain}",
        container,
        "localhost",
        f"{container}.{project_name}.{local_domain}",
        f"{project_name}.{local_domain}",
    ]
    if is_ingress:
        sans.append(f"*.{project_name}.{local_domain}")
    for ip in IP_SANS:
        sans.append(ip)
    return sans


def ensure_ca():
    CERTS.mkdir(parents=True, exist_ok=True)
    if (CERTS / "ca.crt").exists():
        return
    subprocess.check_call([
        STEP, "certificate", "create", "LocalMesh Root CA",
        str(CERTS / "ca.crt"), str(CERTS / "ca.key"),
        "--profile", "root-ca", "--not-after", "87600h",
        "--insecure", "--no-password",
    ])
    os.chmod(CERTS / "ca.key", 0o600)


def issue(container: str, project_name: str, local_domain: str, is_ingress: bool):
    out = CERTS / container
    out.mkdir(exist_ok=True)
    (out / "trust.ca.crt").write_bytes((CERTS / "ca.crt").read_bytes())
    crt, key = out / "id.crt", out / "id.key"
    if crt.exists() and key.exists():
        # Already issued — still ensure perms are container-readable. See
        # the perm-rationale comment further down.
        os.chmod(crt, 0o644)
        os.chmod(key, 0o644)
        return
    san_args = sum(
        [["--san", s] for s in san_list_for(container, project_name, local_domain, is_ingress)],
        [],
    )
    subprocess.check_call([
        STEP, "certificate", "create", container, str(crt), str(key),
        "--profile", "leaf",
        "--ca", str(CERTS / "ca.crt"),
        "--ca-key", str(CERTS / "ca.key"),
        *san_args,
        "--not-after", "87600h",
        "--insecure", "--no-password",
    ])
    # Dev convenience: cert + key are 0644 on the host so containers running
    # under arbitrary uids can read them via bind-mount. Postgres demands
    # 0600 on .key and rejects bind-mounted uid-1000 files outright; the
    # database/ plugin's entrypoint wrapper copies them into the postgres-
    # owned /etc/postgres-ssl/ at startup. Other consumers (rust sqlx, node
    # @grpc/grpc-js, caddy) don't enforce perm checks.
    #
    # TODO: needs_prod_decisions tight key perms (0600 owned by the workload
    # uid) once cert delivery is sidecar / SPIRE-managed instead of bind-mount.
    os.chmod(crt, 0o644)
    os.chmod(key, 0o644)


def mint_certs(project: dict, plugins: list[tuple[str, dict]]):
    ensure_ca()
    pname = project["project_name"]
    ldom = project["local_domain"]
    for container, _port, _expose, exempt, ingress, _owner, _requires_auth in services_iter(project, plugins):
        if exempt:
            continue
        issue(container, pname, ldom, ingress)


# ---------------------------------------------------------------------------
# .env generation (managed section)

def upcase_snake(s: str) -> str:
    return s.upper().replace("-", "_")


def fmt_value(value) -> str:
    """Render a TOML scalar for .env: bools lower-case, everything else str()."""
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


def _existing_env_value(key: str) -> str | None:
    """Read a value from the existing .env (any section, managed or not)."""
    if not ENV_FILE.exists():
        return None
    for line in ENV_FILE.read_text().splitlines():
        if line.startswith(f"{key}="):
            return line.split("=", 1)[1]
    return None


def ensure_secret(key: str, mint) -> str:
    """Return existing .env value for key, or mint + return a new one."""
    existing = _existing_env_value(key)
    if existing:
        return existing
    return mint()


def mint_hex(n_bytes: int = 16) -> str:
    return _secrets.token_hex(n_bytes)


def mint_base64(n_bytes: int = 32) -> str:
    return _secrets.token_urlsafe(n_bytes)


def env_lines(project: dict, plugins: list[tuple[str, dict]]) -> list[str]:
    lines: list[str] = []

    # project.toml top-level scalars → UPCASE_SNAKECASE
    for key, value in project.items():
        if isinstance(value, (str, int, float, bool)):
            lines.append(f"{upcase_snake(key)}={fmt_value(value)}")

    # plugin.toml [identity] blocks → <PLUGIN>_<KEY>. Mesh-exempt status
    # is read from compose labels (see compose_exempts) — not a TOML field.
    for slug, plugin in plugins:
        ident = plugin.get("identity", {})
        prefix = upcase_snake(slug)
        for key, value in ident.items():
            if isinstance(value, (str, int, float, bool)):
                lines.append(f"{prefix}_{upcase_snake(key)}={fmt_value(value)}")

    # [[services]] entries → <CONTAINER>_<KEY>.
    for container, port, expose, _exempt, ingress, _owner, _requires_auth in services_iter(project, plugins):
        prefix = upcase_snake(container)
        lines.append(f"{prefix}_PORT={port}")
        lines.append(f"{prefix}_EXPOSE_VIA_INGRESS={'true' if expose else 'false'}")
        if ingress:
            lines.append(f"{prefix}_INGRESS=true")

    # Auth-plugin secrets. Minted once, preserved across runs.
    lines.append(f"LOCALMESH_OIDC_CLIENT_SECRET={ensure_secret('LOCALMESH_OIDC_CLIENT_SECRET', mint_hex)}")
    lines.append(f"OAUTH2_PROXY_COOKIE_SECRET={ensure_secret('OAUTH2_PROXY_COOKIE_SECRET', mint_base64)}")

    return lines


def write_env(project: dict, plugins: list[tuple[str, dict]]):
    body = "\n".join([MANAGED_START, *env_lines(project, plugins), MANAGED_END]) + "\n"
    if ENV_FILE.exists():
        existing = ENV_FILE.read_text()
        if MANAGED_START in existing and MANAGED_END in existing:
            head, _, rest = existing.partition(MANAGED_START)
            _, _, tail = rest.partition(MANAGED_END)
            tail = tail.lstrip("\n")
            new = head.rstrip("\n") + "\n\n" + body + (tail if tail else "")
        else:
            new = existing.rstrip("\n") + "\n\n" + body
    else:
        new = body
    ENV_FILE.write_text(new)


# ---------------------------------------------------------------------------
# Caddyfile generation

def _ungated_block(upstream: str, exempt: bool) -> list[str]:
    """Caddy site-block body for services that opt out of ingress auth."""
    if exempt:
        return [f"  reverse_proxy {upstream}"]
    return [
        f"  reverse_proxy {upstream} {{",
        f"    transport http {{",
        f"      tls",
        f"      tls_trust_pool file /run/caddy/trust.ca.crt",
        f"      tls_client_auth /run/caddy/id.crt /run/caddy/id.key",
        f"    }}",
        f"  }}",
    ]


def _gated_block(upstream: str, exempt: bool) -> list[str]:
    """Caddy site-block body with forward_auth → oauth2-proxy."""
    upstream_block = (
        [f"    reverse_proxy {upstream}"] if exempt
        else [
            f"    reverse_proxy {upstream} {{",
            f"      transport http {{",
            f"        tls",
            f"        tls_trust_pool file /run/caddy/trust.ca.crt",
            f"        tls_client_auth /run/caddy/id.crt /run/caddy/id.key",
            f"      }}",
            f"    }}",
        ]
    )
    return [
        # NEW: open a span per request via the built-in tracing module.
        # OTLP target comes from OTEL_EXPORTER_OTLP_ENDPOINT env on
        # the caddy container. The span is shared with the rest of the
        # handler chain, so enduser_attrs (below) can stamp it.
        f"  tracing {{",
        f"    span ingress",
        f"  }}",
        f"  handle /oauth2/* {{",
        f"    reverse_proxy http://oauth2-proxy:4180",
        f"  }}",
        f"  handle {{",
        f"    forward_auth http://oauth2-proxy:4180 {{",
        f"      uri /oauth2/auth",
        f"      copy_headers {{",
        f"        X-Auth-Request-User>X-Forwarded-User",
        f"        X-Auth-Request-Email>X-Forwarded-Email",
        f"        X-Auth-Request-Preferred-Username>X-Forwarded-Preferred-Username",
        f"        X-Auth-Request-Access-Token>Authorization",
        f"      }}",
        f"      @error status 401",
        f"      handle_response @error {{",
        f"        redir https://{{http.request.hostport}}/oauth2/sign_in?rd={{scheme}}://{{http.request.hostport}}{{http.request.orig_uri}} 302",
        f"      }}",
        f"    }}",
        # NEW: stamp enduser.* on the active OTel span from the trusted headers.
        f"    enduser_attrs {{",
        f"      X-Forwarded-User enduser.id",
        f"      X-Forwarded-Email enduser.email",
        f"      X-Forwarded-Preferred-Username enduser.preferred_username",
        f"    }}",
        *upstream_block,
        f"  }}",
    ]


def caddyfile_lines(project: dict, plugins: list[tuple[str, dict]]) -> list[str]:
    pname = project["project_name"]
    ldom = project["local_domain"]
    lines = [
        f"# Auto-generated by scripts/secrets-gen.py. Do not edit by hand.",
        f"# Re-run `make certs` to regenerate.",
        "",
        "{",
        # Bind admin (which serves /metrics) to all interfaces so the
        # otel-collector can scrape from inside the docker network. Host
        # exposure is still locked down via compose `ports: 127.0.0.1:2019:2019`.
        "  admin :2019",
        "  # We supply certs via the per-site `tls` directive (read from",
        "  # /run/caddy/id.{crt,key}). Disable Caddy's automatic cert",
        "  # acquisition so it doesn't issue its own and override ours.",
        "  auto_https disable_certs",
        # NEW: Caddy v2 refuses non-standard directives without explicit
        # ordering. `tracing` opens a span and must wrap everything;
        # `enduser_attrs` (plugin/enduser_attrs/) reads that span and
        # stamps attrs, so it must run before reverse_proxy hops upstream.
        "  order tracing first",
        "  order enduser_attrs before reverse_proxy",
        # Caddy auto-exposes Prometheus metrics at /metrics on the admin
        # endpoint (caddy:2019), no opt-in required. The otel-collector's
        # prometheus receiver scrapes them — see service_catalog/observability.
        "  log {",
        "    output stdout",
        "    format json",
        "  }",
        "}",
        "",
    ]
    for container, port, expose, exempt, ingress, _owner, requires_auth in services_iter(project, plugins):
        if not expose or ingress:
            continue
        host = f"{container}.{pname}.{ldom}"
        upstream_scheme = "http" if exempt else "https"
        upstream = f"{upstream_scheme}://{container}:{port}"
        lines.append(f"{host}:8443 {{")
        lines.append(f"  tls /run/caddy/id.crt /run/caddy/id.key")
        if requires_auth:
            lines.extend(_gated_block(upstream, exempt))
        else:
            lines.extend(_ungated_block(upstream, exempt))
        lines.append(f"  log {{")
        lines.append(f"    output stdout")
        lines.append(f"    format json")
        lines.append(f"  }}")
        lines.append(f"}}")
        lines.append("")
    return lines


def write_caddyfile(project: dict, plugins: list[tuple[str, dict]]):
    CADDYFILE.parent.mkdir(parents=True, exist_ok=True)
    CADDYFILE.write_text("\n".join(caddyfile_lines(project, plugins)))


# ---------------------------------------------------------------------------
# Dex connector generation (auth plugin)

AUTH_DIR = CATALOG / "auth"
DEX_BASE = AUTH_DIR / "dex.yaml"
USERS_FILE = SECRETS / "users.yaml"
DEX_GENERATED = AUTH_DIR / "dex.yaml.generated"


# bcrypt(cost=10) hash of the dev password "dev". Every staticPasswords
# entry shares this hash — dev UX, not a real credential. Regenerate via:
#   docker run --rm httpd:2.4-alpine htpasswd -bnBC 10 "" dev | cut -d: -f2
DEV_PASSWORD_BCRYPT = "$2y$10$N.b.upq/.fOOUGbkQ2uy8u7mCgjQJA6hUnedHSVy4zcL4c23Ug7R2"


def write_dex_connectors():
    """Emit Dex's full config — base from dex.yaml + one staticPasswords entry per dev user.

    Dex's CLI accepts exactly one config file, so we merge the hand-written
    base (dex.yaml — issuer, storage, staticClients, enablePasswordDB) with
    the generated staticPasswords block here and write a single
    dex.yaml.generated. Compose bind-mounts only the merged file; dex.yaml
    is read by this script as source, never by dex directly.

    Uses Dex's built-in local connector (enablePasswordDB in dex.yaml).
    mockCallback was the original choice but it's a no-config connector
    that always returns Kilgore Trout — and its identity doesn't include
    a preferred_username claim, which broke the www banner.

    No-op when the auth plugin isn't present (catalog without auth/), so
    this is safe to call unconditionally from main().
    """
    if not AUTH_DIR.exists() or not DEX_BASE.exists() or not USERS_FILE.exists():
        return
    users = (yaml.safe_load(USERS_FILE.read_text()) or {}).get("users") or []

    # Dex doesn't reliably env-expand config values across versions, so we
    # resolve $PROJECT_NAME / $LOCAL_DOMAIN / $LOCALMESH_OIDC_CLIENT_SECRET
    # here at generation time. Compose passes them via env: anyway, but Dex
    # receives a fully literal config so behavior doesn't depend on its
    # expansion quirks.
    project, _ = load_manifests()
    expansions = {
        "PROJECT_NAME": project["project_name"],
        "LOCAL_DOMAIN": project["local_domain"],
        "LOCALMESH_OIDC_CLIENT_SECRET": _existing_env_value("LOCALMESH_OIDC_CLIENT_SECRET") or "",
    }
    base = DEX_BASE.read_text()
    for var, val in expansions.items():
        base = base.replace(f"${{{var}}}", val).replace(f"${var}", val)

    out = [
        "# Auto-generated by scripts/secrets-gen.py. Do not edit by hand.",
        "# Source: service_catalog/auth/dex.yaml + .secrets/users.yaml.",
        "# Env-vars ($PROJECT_NAME, $LOCAL_DOMAIN, $LOCALMESH_OIDC_CLIENT_SECRET)",
        "# are pre-expanded here (Dex's runtime env-expansion is unreliable).",
        "# Re-run `make certs` to regenerate.",
        "",
        base.rstrip(),
        "",
        "staticPasswords:",
    ]
    for u in users:
        out.extend([
            f"  - email: {u['email']}",
            f"    hash: '{DEV_PASSWORD_BCRYPT}'",
            f"    username: \"{u['name']}\"",
            f"    userID: {u['id']}",
        ])
    DEX_GENERATED.write_text("\n".join(out) + "\n")


# ---------------------------------------------------------------------------

def main():
    if not PROJECT_TOML.exists():
        print(f"ERROR: {PROJECT_TOML} not found.", file=sys.stderr)
        sys.exit(1)
    project, plugins = load_manifests()
    mint_certs(project, plugins)
    write_env(project, plugins)
    write_caddyfile(project, plugins)
    write_dex_connectors()
    print("LocalMesh secrets: regenerated.")


if __name__ == "__main__":
    main()
