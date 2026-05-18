#!/usr/bin/env python3
"""LocalMesh secrets bootstrap.

Reads `project.toml` (repo root) and every `service_catalog/*/plugin.toml`,
then:

  1. Mints the LocalMesh CA at `.secrets/certs/ca.{crt,key}` (one-shot).
  2. Mints a bidirectional leaf cert per declared service at
     `.secrets/certs/<container>/{trust.ca.crt, id.crt, id.key}` — skipped if
     the plugin's `[identity].mesh_exempt = true`.
  3. Writes `.env` with UPCASE_SNAKECASE versions of every TOML key (in a
     managed section between `# >>> secrets-gen managed` markers).
  4. Generates `service_catalog/caddy/Caddyfile.generated` with one site
     block per service that has `expose_via_ingress = true` — mTLS upstream
     for mesh-participating services, plaintext for mesh-exempt ones.

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
import subprocess
import sys
import tomllib

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


def services_iter(project: dict, plugins: list[tuple[str, dict]]):
    """Yield (container, port, expose_via_ingress, mesh_exempt, ingress, owner_slug)."""
    for svc in project.get("services", []):
        yield (
            svc["container"],
            svc["port"],
            svc.get("expose_via_ingress", False),
            False,                                       # project-level services never mesh-exempt
            svc.get("ingress", False),
            "project",
        )
    for slug, plugin in plugins:
        exempt = plugin.get("identity", {}).get("mesh_exempt", False)
        for svc in plugin.get("services", []):
            yield (
                svc["container"],
                svc["port"],
                svc.get("expose_via_ingress", False),
                exempt,
                svc.get("ingress", False),
                slug,
            )


# ---------------------------------------------------------------------------
# Cert generation

def san_list_for(container: str, project_name: str, local_domain: str, is_ingress: bool) -> list[str]:
    sans = [
        f"URI:spiffe://{container}.{project_name}.{local_domain}",
        f"DNS:{container}",
        "DNS:localhost",
        f"DNS:{container}.{project_name}.{local_domain}",
        f"DNS:{project_name}.{local_domain}",
    ]
    if is_ingress:
        sans.append(f"DNS:*.{project_name}.{local_domain}")
    for ip in IP_SANS:
        sans.append(f"IP:{ip}")
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
    os.chmod(key, 0o600)


def mint_certs(project: dict, plugins: list[tuple[str, dict]]):
    ensure_ca()
    pname = project["project_name"]
    ldom = project["local_domain"]
    for container, _port, _expose, exempt, ingress, _owner in services_iter(project, plugins):
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


def env_lines(project: dict, plugins: list[tuple[str, dict]]) -> list[str]:
    lines: list[str] = []

    # project.toml top-level scalars → UPCASE_SNAKECASE
    for key, value in project.items():
        if isinstance(value, (str, int, float, bool)):
            lines.append(f"{upcase_snake(key)}={fmt_value(value)}")

    # plugin.toml [identity] blocks → <PLUGIN>_<KEY>. The mesh_exempt flag
    # lives here (per-plugin), not on individual services.
    for slug, plugin in plugins:
        ident = plugin.get("identity", {})
        prefix = upcase_snake(slug)
        for key, value in ident.items():
            if isinstance(value, (str, int, float, bool)):
                lines.append(f"{prefix}_{upcase_snake(key)}={fmt_value(value)}")

    # [[services]] entries → <CONTAINER>_<KEY>. mesh_exempt is plugin-level
    # (above), not duplicated per-service.
    for container, port, expose, _exempt, ingress, _owner in services_iter(project, plugins):
        prefix = upcase_snake(container)
        lines.append(f"{prefix}_PORT={port}")
        lines.append(f"{prefix}_EXPOSE_VIA_INGRESS={'true' if expose else 'false'}")
        if ingress:
            lines.append(f"{prefix}_INGRESS=true")

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

def caddyfile_lines(project: dict, plugins: list[tuple[str, dict]]) -> list[str]:
    pname = project["project_name"]
    ldom = project["local_domain"]
    lines = [
        f"# Auto-generated by scripts/secrets-gen.py. Do not edit by hand.",
        f"# Re-run `make certs` to regenerate.",
        "",
        "{",
        "  admin localhost:2019",
        "  # We supply certs via the per-site `tls` directive (read from",
        "  # /run/caddy/id.{crt,key}). Disable Caddy's automatic cert",
        "  # acquisition so it doesn't issue its own and override ours.",
        "  auto_https disable_certs",
        "  log {",
        "    output stdout",
        "    format json",
        "  }",
        "}",
        "",
    ]
    for container, port, expose, exempt, ingress, _owner in services_iter(project, plugins):
        if not expose or ingress:
            continue
        host = f"{container}.{pname}.{ldom}"
        upstream_scheme = "http" if exempt else "https"
        upstream = f"{upstream_scheme}://{container}:{port}"
        lines.append(f"{host}:8443 {{")
        lines.append(f"  tls /run/caddy/id.crt /run/caddy/id.key")
        if exempt:
            lines.append(f"  reverse_proxy {upstream}")
        else:
            lines.append(f"  reverse_proxy {upstream} {{")
            lines.append(f"    transport http {{")
            lines.append(f"      tls")
            lines.append(f"      tls_trust_pool file /run/caddy/trust.ca.crt")
            lines.append(f"      tls_client_auth /run/caddy/id.crt /run/caddy/id.key")
            lines.append(f"    }}")
            lines.append(f"  }}")
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

def main():
    if not PROJECT_TOML.exists():
        print(f"ERROR: {PROJECT_TOML} not found.", file=sys.stderr)
        sys.exit(1)
    project, plugins = load_manifests()
    mint_certs(project, plugins)
    write_env(project, plugins)
    write_caddyfile(project, plugins)
    print("LocalMesh secrets: regenerated.")


if __name__ == "__main__":
    main()
