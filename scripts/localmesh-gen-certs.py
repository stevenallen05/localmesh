#!/usr/bin/env python3
"""Generate the LocalMesh CA + per-plugin leaf certs.

Reads `service_catalog/*/plugin.toml`'s `[[certs]]` entries; mints certs
offline via the vendored `tools/step` CLI. Conventions for CN, SAN, EKU,
and lifetime are baked in here — one place to change for the whole catalog.

See docs/superpowers/specs/2026-05-18-localmesh-service-mesh-design.md §6+§7.

TODO: needs_prod_decisions IP SAN list is intentionally over-permissive for
dev convenience. Prod uses DNS-based identity exclusively — cert-manager /
SPIRE SVIDs carry no IP SANs at all.

TODO: needs_prod_decisions tighten EKU per client/server bools (step template).
The bools in plugin.toml are intent docs today; step's `leaf` profile applies
both serverAuth and clientAuth regardless.

TODO: needs_prod_decisions cert lifespan default — 10y for dev convenience;
prod uses short-lived SVIDs (~1h) with SPIRE-managed rotation.
"""
import os
import pathlib
import subprocess
import sys
import tomllib

CERTS = pathlib.Path(os.environ.get("CERTS_DIR", ".localmesh/certs"))
CATALOG = pathlib.Path(os.environ.get("CATALOG_DIR", "service_catalog"))
PROJECT = os.environ.get("PROJECT_NAME", "localmesh")
DOMAIN = os.environ.get("INTERNAL_DOMAIN", "tw-demo.local")
STEP = "./tools/step"

# Over-permissive on purpose. See TODO above.
IP_SANS = [
    "127.0.0.1", "::1", "0.0.0.0",
    "10.0.0.1", "172.17.0.1", "172.18.0.1", "172.19.0.1", "172.20.0.1",
    "192.168.65.1", "192.168.1.1",
]


def san_list_for(cid: str) -> list[str]:
    """Assemble the SAN list for a workload by convention."""
    sans = [
        f"URI:spiffe://{PROJECT}.{DOMAIN}/ns/default/sa/{cid}",
        f"DNS:{cid}",
        "DNS:localhost",
        f"DNS:{cid}.{PROJECT}.{DOMAIN}",
        f"DNS:{PROJECT}.{DOMAIN}",
    ]
    if cid == "caddy":
        sans.append(f"DNS:*.{PROJECT}.{DOMAIN}")
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


def discover_plugins():
    """Yield {slug, certs} dicts from service_catalog/**/plugin.toml.

    Plugins without a `[[certs]]` block are skipped (e.g., auth_shim).
    """
    for manifest in CATALOG.rglob("plugin.toml"):
        try:
            plugin = tomllib.loads(manifest.read_text())
        except (tomllib.TOMLDecodeError, OSError) as e:
            print(f"WARN: skipping {manifest}: {e}", file=sys.stderr)
            continue
        if "certs" not in plugin:
            continue
        yield {"slug": manifest.parent.name, "certs": plugin["certs"]}


def issue(out: pathlib.Path, cert: dict):
    cid = cert["id"]
    crt, key = out / f"id.{cid}.crt", out / f"id.{cid}.key"
    if crt.exists() and key.exists():
        return
    san_args = sum([["--san", s] for s in san_list_for(cid)], [])
    subprocess.check_call([
        STEP, "certificate", "create", cid, str(crt), str(key),
        "--profile", "leaf",
        "--ca", str(CERTS / "ca.crt"),
        "--ca-key", str(CERTS / "ca.key"),
        *san_args,
        "--not-after", "87600h",
        "--insecure", "--no-password",
    ])
    os.chmod(key, 0o600)


def main():
    ensure_ca()
    for plugin in discover_plugins():
        out = CERTS / plugin["slug"]
        out.mkdir(exist_ok=True)
        (out / "trust.ca.crt").write_bytes((CERTS / "ca.crt").read_bytes())
        for cert in plugin["certs"]:
            issue(out, cert)


if __name__ == "__main__":
    main()
