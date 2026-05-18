"""Sanity tests for localmesh-gen-certs.py.

Doesn't invoke `step` directly; imports the script's helpers and checks
the plumbing (SAN-list assembly, plugin discovery, idempotency).
"""
import importlib.util
import pathlib
import sys


def _load_module(path):
    spec = importlib.util.spec_from_file_location("certs", path)
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "localmesh-gen-certs.py"


def test_san_list_includes_spiffe_uri_and_localhost_and_internal_domain(monkeypatch, tmp_path):
    monkeypatch.setenv("PROJECT_NAME", "demo")
    monkeypatch.setenv("INTERNAL_DOMAIN", "tw-demo.local")
    monkeypatch.setenv("CERTS_DIR", str(tmp_path))
    mod = _load_module(str(SCRIPT))
    sans = mod.san_list_for("server")
    assert any(
        s.startswith("URI:spiffe://demo.tw-demo.local/ns/default/sa/server")
        for s in sans
    )
    assert "DNS:server" in sans
    assert "DNS:localhost" in sans
    assert "DNS:server.demo.tw-demo.local" in sans
    assert "DNS:demo.tw-demo.local" in sans
    assert "IP:127.0.0.1" in sans
    assert "IP:::1" in sans


def test_san_list_for_caddy_adds_wildcard(monkeypatch, tmp_path):
    monkeypatch.setenv("PROJECT_NAME", "demo")
    monkeypatch.setenv("INTERNAL_DOMAIN", "tw-demo.local")
    monkeypatch.setenv("CERTS_DIR", str(tmp_path))
    mod = _load_module(str(SCRIPT))
    sans = mod.san_list_for("caddy")
    assert "DNS:*.demo.tw-demo.local" in sans


def test_discover_plugins_finds_only_those_with_certs_blocks(tmp_path, monkeypatch):
    catalog = tmp_path / "service_catalog"
    plugin = catalog / "caddy"
    plugin.mkdir(parents=True)
    (plugin / "plugin.toml").write_text(
        '[identity]\nmodule_name = "caddy"\nowned_by = "x@y.z"\n\n'
        '[[certs]]\nid = "caddy"\nserver = true\nclient = true\n'
    )
    other = catalog / "auth_shim"
    other.mkdir()
    (other / "plugin.toml").write_text(
        '[identity]\nmodule_name="auth_shim"\nowned_by="x@y.z"\n'
    )
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("PROJECT_NAME", "demo")
    monkeypatch.setenv("INTERNAL_DOMAIN", "tw-demo.local")
    monkeypatch.setenv("CERTS_DIR", str(tmp_path / ".localmesh" / "certs"))
    monkeypatch.setenv("CATALOG_DIR", str(catalog))
    mod = _load_module(str(SCRIPT))
    plugins = list(mod.discover_plugins())
    slugs = {p["slug"] for p in plugins}
    assert slugs == {"caddy"}    # auth_shim has no [[certs]], gets filtered
