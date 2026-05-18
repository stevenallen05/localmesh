"""Unit tests for secrets-gen.py.

The cert-minting path needs a real `step` binary + a real filesystem, so
those are exercised by the integration smoke (make certs). Tests here
cover the pure logic — SAN assembly, env-var name conversion, env-file
managed-section roundtrip, Caddyfile generation.
"""
import importlib.util
import pathlib


SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "secrets-gen.py"


def _load(monkeypatch, repo_root):
    monkeypatch.setenv("REPO_ROOT", str(repo_root))
    spec = importlib.util.spec_from_file_location("secrets_gen", SCRIPT)
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


def _seed(repo, project_toml, plugins):
    """Create project.toml + service_catalog/<slug>/plugin.toml fixtures."""
    (repo / "project.toml").write_text(project_toml)
    catalog = repo / "service_catalog"
    catalog.mkdir()
    for slug, content in plugins.items():
        d = catalog / slug
        d.mkdir()
        (d / "plugin.toml").write_text(content)


def test_upcase_snake(tmp_path, monkeypatch):
    mod = _load(monkeypatch, tmp_path)
    assert mod.upcase_snake("project_name") == "PROJECT_NAME"
    assert mod.upcase_snake("auth-shim") == "AUTH_SHIM"
    assert mod.upcase_snake("Postgres-Exporter") == "POSTGRES_EXPORTER"


def test_san_list_includes_spiffe_dns_localhost_and_ips(tmp_path, monkeypatch):
    mod = _load(monkeypatch, tmp_path)
    sans = mod.san_list_for("server", "demo", "lvh.me", is_ingress=False)
    # Bare values — step CLI autodetects URI / IP / DNS from value shape.
    assert "spiffe://server.demo.lvh.me" in sans
    assert "server" in sans
    assert "localhost" in sans
    assert "server.demo.lvh.me" in sans
    assert "demo.lvh.me" in sans
    assert "127.0.0.1" in sans
    assert "::1" in sans
    assert "*.demo.lvh.me" not in sans


def test_san_list_for_ingress_adds_wildcard(tmp_path, monkeypatch):
    mod = _load(monkeypatch, tmp_path)
    sans = mod.san_list_for("caddy", "demo", "lvh.me", is_ingress=True)
    assert "*.demo.lvh.me" in sans


def test_env_lines_strict_upcase_snakecase(tmp_path, monkeypatch):
    _seed(
        tmp_path,
        project_toml=(
            'project_name      = "demo"\n'
            'tech_lead_email   = "x@y.z"\n'
            'local_domain      = "lvh.me"\n'
            '\n'
            '[[services]]\n'
            'container          = "www"\n'
            'port               = 3443\n'
            'expose_via_ingress = true\n'
        ),
        plugins={
            "caddy": (
                '[identity]\n'
                'module_name = "caddy"\n'
                'owned_by    = "sre@example.com"\n'
                '\n'
                '[[services]]\n'
                'container = "caddy"\n'
                'port      = 8443\n'
                'ingress   = true\n'
                'expose_via_ingress = false\n'
            ),
            "obs": (
                '[identity]\n'
                'module_name = "observability"\n'
                'owned_by    = "sre@example.com"\n'
                'mesh_exempt = true\n'
                '\n'
                '[[services]]\n'
                'container          = "grafana"\n'
                'port               = 3000\n'
                'expose_via_ingress = true\n'
            ),
        },
    )
    mod = _load(monkeypatch, tmp_path)
    project, plugins = mod.load_manifests()
    out = mod.env_lines(project, plugins)
    text = "\n".join(out)
    assert "PROJECT_NAME=demo" in text
    assert "TECH_LEAD_EMAIL=x@y.z" in text
    assert "LOCAL_DOMAIN=lvh.me" in text
    assert "CADDY_MODULE_NAME=caddy" in text
    assert "CADDY_OWNED_BY=sre@example.com" in text
    assert "OBS_MESH_EXEMPT=True" in text or "OBS_MESH_EXEMPT=true" in text
    assert "WWW_PORT=3443" in text
    assert "WWW_EXPOSE_VIA_INGRESS=true" in text
    assert "CADDY_PORT=8443" in text
    assert "CADDY_INGRESS=true" in text
    assert "GRAFANA_PORT=3000" in text
    assert "GRAFANA_EXPOSE_VIA_INGRESS=true" in text


def test_env_managed_section_roundtrip(tmp_path, monkeypatch):
    _seed(
        tmp_path,
        project_toml=(
            'project_name = "demo"\n'
            'local_domain = "lvh.me"\n'
        ),
        plugins={},
    )
    user_top = "# user file\nIMAGE_TAG=feat-x\n"
    (tmp_path / ".env").write_text(user_top)

    mod = _load(monkeypatch, tmp_path)
    project, plugins = mod.load_manifests()
    mod.write_env(project, plugins)
    text = (tmp_path / ".env").read_text()
    assert "# user file" in text
    assert "IMAGE_TAG=feat-x" in text
    assert "# >>> secrets-gen managed" in text
    assert "PROJECT_NAME=demo" in text

    # Second run preserves head + replaces managed section.
    mod.write_env(project, plugins)
    text2 = (tmp_path / ".env").read_text()
    assert text2.count("# >>> secrets-gen managed") == 1
    assert "IMAGE_TAG=feat-x" in text2


def test_caddyfile_emits_route_for_exposed_services(tmp_path, monkeypatch):
    _seed(
        tmp_path,
        project_toml=(
            'project_name = "demo"\n'
            'local_domain = "lvh.me"\n'
            '\n'
            '[[services]]\n'
            'container          = "www"\n'
            'port               = 3443\n'
            'expose_via_ingress = true\n'
        ),
        plugins={
            "caddy": (
                '[identity]\nmodule_name = "caddy"\nowned_by = "sre@example.com"\n'
                '[[services]]\ncontainer = "caddy"\nport = 8443\ningress = true\n'
            ),
            "obs": (
                '[identity]\nmodule_name = "observability"\nowned_by = "sre@example.com"\nmesh_exempt = true\n'
                '[[services]]\ncontainer = "grafana"\nport = 3000\nexpose_via_ingress = true\n'
            ),
        },
    )
    mod = _load(monkeypatch, tmp_path)
    project, plugins = mod.load_manifests()
    text = "\n".join(mod.caddyfile_lines(project, plugins))
    assert "www.demo.lvh.me:8443" in text
    assert "grafana.demo.lvh.me:8443" in text
    assert "caddy.demo.lvh.me:8443" not in text     # ingress=true: no self-route
    # Mesh-exempt grafana: plaintext upstream
    assert "reverse_proxy http://grafana:3000" in text
    # Mesh-participating www: mTLS upstream
    assert "reverse_proxy https://www:3443" in text
    assert "tls_client_auth /run/caddy/id.crt /run/caddy/id.key" in text
