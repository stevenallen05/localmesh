"""Sanity tests for localmesh-gen-plugin-env.py."""
import importlib.util
import pathlib


SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "localmesh-gen-plugin-env.py"


def _load(path):
    spec = importlib.util.spec_from_file_location("envgen", path)
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


def test_renders_per_plugin_vars(tmp_path, monkeypatch):
    catalog = tmp_path / "service_catalog"
    (catalog / "caddy").mkdir(parents=True)
    (catalog / "caddy" / "plugin.toml").write_text(
        '[identity]\nmodule_name = "caddy"\nowned_by = "sre@example.com"\n'
    )
    (catalog / "auth_shim").mkdir()
    (catalog / "auth_shim" / "plugin.toml").write_text(
        '[identity]\nmodule_name = "auth_shim"\nowned_by = "sec@example.com"\n'
    )
    out = tmp_path / ".env.localmesh"
    monkeypatch.setenv("CATALOG_DIR", str(catalog))
    monkeypatch.setenv("OUT_FILE", str(out))
    mod = _load(str(SCRIPT))
    mod.main()
    text = out.read_text()
    assert "CADDY_MODULE_NAME=caddy" in text
    assert "CADDY_OWNED_BY=sre@example.com" in text
    assert "AUTH_SHIM_MODULE_NAME=auth_shim" in text
    assert "AUTH_SHIM_OWNED_BY=sec@example.com" in text


def test_skips_plugins_without_identity_block(tmp_path, monkeypatch):
    catalog = tmp_path / "service_catalog"
    (catalog / "weird").mkdir(parents=True)
    (catalog / "weird" / "plugin.toml").write_text('[[certs]]\nid = "weird"\n')
    out = tmp_path / ".env.localmesh"
    monkeypatch.setenv("CATALOG_DIR", str(catalog))
    monkeypatch.setenv("OUT_FILE", str(out))
    mod = _load(str(SCRIPT))
    mod.main()
    text = out.read_text()
    assert "WEIRD_" not in text


def test_idempotent_on_rerun(tmp_path, monkeypatch):
    catalog = tmp_path / "service_catalog"
    (catalog / "caddy").mkdir(parents=True)
    (catalog / "caddy" / "plugin.toml").write_text(
        '[identity]\nmodule_name = "caddy"\nowned_by = "sre@example.com"\n'
    )
    out = tmp_path / ".env.localmesh"
    monkeypatch.setenv("CATALOG_DIR", str(catalog))
    monkeypatch.setenv("OUT_FILE", str(out))
    mod = _load(str(SCRIPT))
    mod.main()
    first = out.read_text()
    mod.main()
    second = out.read_text()
    assert first == second
