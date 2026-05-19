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


def _seed(repo, project_toml, plugins, composes=None):
    """Create project.toml + per-plugin plugin.toml [+ docker-compose.yml]."""
    (repo / "project.toml").write_text(project_toml)
    catalog = repo / "service_catalog"
    catalog.mkdir()
    for slug, content in plugins.items():
        d = catalog / slug
        d.mkdir()
        (d / "plugin.toml").write_text(content)
    for slug, content in (composes or {}).items():
        (catalog / slug / "docker-compose.yml").write_text(content)


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
    # mesh_exempt is not a TOML field — it lives in compose labels, not .env.
    assert "MESH_EXEMPT" not in text
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
                '[identity]\nmodule_name = "observability"\nowned_by = "sre@example.com"\n'
                '[[services]]\ncontainer = "grafana"\nport = 3000\nexpose_via_ingress = true\n'
            ),
        },
        composes={
            # mesh-exempt declared on compose labels — the source of truth.
            "obs": (
                'services:\n'
                '  grafana:\n'
                '    image: grafana/grafana:12.4.3\n'
                '    labels:\n'
                '      mesh.exempt: "true"\n'
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


def test_caddyfile_emits_gated_by_default(tmp_path, monkeypatch):
    """Services without `requires_auth` explicitly set get the gated template."""
    _seed(
        tmp_path,
        project_toml=(
            'project_name = "demo"\nlocal_domain = "lvh.me"\n\n'
            '[[services]]\ncontainer = "www"\nport = 3443\nexpose_via_ingress = true\n'
        ),
        plugins={
            "caddy": (
                '[identity]\nmodule_name = "caddy"\nowned_by = "sre@example.com"\n'
                '[[services]]\ncontainer = "caddy"\nport = 8443\ningress = true\n'
            ),
        },
    )
    mod = _load(monkeypatch, tmp_path)
    project, plugins = mod.load_manifests()
    text = "\n".join(mod.caddyfile_lines(project, plugins))
    # Gated default emits the forward_auth marker block.
    assert "forward_auth http://oauth2-proxy:4180" in text
    # Global ordering directives for the custom + built-in OTel directives.
    assert "order enduser_attrs before reverse_proxy" in text
    assert "order tracing first" in text
    # Site-level tracing directive (Caddy v2 rejects tracing as a global option).
    assert "tracing {" in text
    assert "span ingress" in text
    # enduser_attrs directive with the three header→attribute mappings.
    assert "enduser_attrs {" in text
    assert "X-Forwarded-User enduser.id" in text
    assert "X-Forwarded-Email enduser.email" in text
    assert "X-Forwarded-Preferred-Username enduser.preferred_username" in text


def test_caddyfile_requires_auth_false_emits_ungated_template(tmp_path, monkeypatch):
    """Explicit `requires_auth = false` opts out of the forward_auth block."""
    _seed(
        tmp_path,
        project_toml=(
            'project_name = "demo"\nlocal_domain = "lvh.me"\n\n'
            '[[services]]\ncontainer = "www"\nport = 3443\nexpose_via_ingress = true\n'
            'requires_auth = false\n'
        ),
        plugins={
            "caddy": (
                '[identity]\nmodule_name = "caddy"\nowned_by = "sre@example.com"\n'
                '[[services]]\ncontainer = "caddy"\nport = 8443\ningress = true\n'
            ),
        },
    )
    mod = _load(monkeypatch, tmp_path)
    project, plugins = mod.load_manifests()
    text = "\n".join(mod.caddyfile_lines(project, plugins))
    assert "forward_auth" not in text
    assert "reverse_proxy https://www:3443" in text
    # Ungated services don't carry per-site enduser_attrs or tracing blocks.
    # Match the opening-brace form so the global `order` directives (which
    # mention both directive names unconditionally) aren't false positives.
    assert "enduser_attrs {" not in text
    assert "span ingress" not in text


def test_dex_yaml_generated_merges_base_plus_one_static_password_per_user(tmp_path, monkeypatch):
    """Reads dex.yaml + users.yaml, writes a merged dex.yaml.generated."""
    _seed(
        tmp_path,
        project_toml='project_name = "demo"\nlocal_domain = "lvh.me"\n',
        plugins={
            "auth": (
                '[identity]\nmodule_name = "auth"\nowned_by = "sre@example.com"\n'
                '[[services]]\ncontainer = "dex"\nport = 5556\nexpose_via_ingress = true\nrequires_auth = false\n'
            ),
        },
        composes={
            "auth": 'services:\n  dex:\n    image: dex\n    labels:\n      mesh.exempt: "true"\n',
        },
    )
    # dex.yaml lives in the plugin dir; dex's CLI takes only one config file,
    # so secrets-gen.py merges base + staticPasswords into dex.yaml.generated.
    (tmp_path / "service_catalog" / "auth" / "dex.yaml").write_text(
        'issuer: https://dex.demo.lvh.me:8443/dex\n'
        'storage:\n  type: memory\n'
        'enablePasswordDB: true\n'
    )
    secrets = tmp_path / ".secrets"
    secrets.mkdir()
    (secrets / "users.yaml").write_text(
        'users:\n'
        '  - { id: alice,   email: alice@example.invalid,   name: "Alice Example" }\n'
        '  - { id: bob,     email: bob@example.invalid,     name: "Bob Tester" }\n'
    )
    mod = _load(monkeypatch, tmp_path)
    project, plugins = mod.load_manifests()
    mod.write_dex_connectors()
    generated = (tmp_path / "service_catalog" / "auth" / "dex.yaml.generated").read_text()
    # Base block preserved verbatim.
    assert "issuer: https://dex.demo.lvh.me:8443/dex" in generated
    assert "type: memory" in generated
    assert "enablePasswordDB: true" in generated
    # staticPasswords block appended below.
    assert "staticPasswords:" in generated
    assert "email: alice@example.invalid" in generated
    assert "userID: alice" in generated
    assert 'username: "Alice Example"' in generated
    assert "email: bob@example.invalid" in generated
    assert "userID: bob" in generated
    # Shared bcrypt hash present (one per user).
    assert generated.count(mod.DEV_PASSWORD_BCRYPT) == 2


def test_dex_yaml_generated_skipped_when_auth_plugin_absent(tmp_path, monkeypatch):
    """If service_catalog/auth/ doesn't exist, no dex.yaml.generated is written."""
    _seed(
        tmp_path,
        project_toml='project_name = "demo"\nlocal_domain = "lvh.me"\n',
        plugins={},
    )
    secrets = tmp_path / ".secrets"
    secrets.mkdir()
    (secrets / "users.yaml").write_text('users:\n  - { id: a, email: a@x, name: A }\n')
    mod = _load(monkeypatch, tmp_path)
    mod.write_dex_connectors()    # should be a no-op
    assert not (tmp_path / "service_catalog" / "auth" / "dex.yaml.generated").exists()


def test_oidc_client_secret_minted_and_persisted(tmp_path, monkeypatch):
    """First run mints LOCALMESH_OIDC_CLIENT_SECRET; second run preserves it."""
    _seed(
        tmp_path,
        project_toml='project_name = "demo"\nlocal_domain = "lvh.me"\n',
        plugins={},
    )
    mod = _load(monkeypatch, tmp_path)
    project, plugins = mod.load_manifests()
    mod.write_env(project, plugins)
    text1 = (tmp_path / ".env").read_text()
    assert "LOCALMESH_OIDC_CLIENT_SECRET=" in text1
    secret_line_1 = [l for l in text1.splitlines() if l.startswith("LOCALMESH_OIDC_CLIENT_SECRET=")][0]
    assert len(secret_line_1.split("=", 1)[1]) >= 32

    mod.write_env(project, plugins)
    text2 = (tmp_path / ".env").read_text()
    secret_line_2 = [l for l in text2.splitlines() if l.startswith("LOCALMESH_OIDC_CLIENT_SECRET=")][0]
    assert secret_line_1 == secret_line_2     # stable across runs


def test_cookie_secret_minted_and_persisted(tmp_path, monkeypatch):
    """OAUTH2_PROXY_COOKIE_SECRET minted once, preserved on re-run."""
    _seed(
        tmp_path,
        project_toml='project_name = "demo"\nlocal_domain = "lvh.me"\n',
        plugins={},
    )
    mod = _load(monkeypatch, tmp_path)
    project, plugins = mod.load_manifests()
    mod.write_env(project, plugins)
    text1 = (tmp_path / ".env").read_text()
    cookie_1 = [l for l in text1.splitlines() if l.startswith("OAUTH2_PROXY_COOKIE_SECRET=")][0]
    assert len(cookie_1.split("=", 1)[1]) >= 32

    mod.write_env(project, plugins)
    text2 = (tmp_path / ".env").read_text()
    cookie_2 = [l for l in text2.splitlines() if l.startswith("OAUTH2_PROXY_COOKIE_SECRET=")][0]
    assert cookie_1 == cookie_2
