"""Unit tests for secrets-gen.py.

Cert minting + `.env` writing have moved to the Go CLI
(./localmesh_src/cmd/localmesh/); tests here cover only what remains in
this script — Caddyfile generation, dex.yaml merge.

TODO: needs_prod_decisions rename test file for transitional helper
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


