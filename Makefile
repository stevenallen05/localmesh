# Begin ops team responsibility

.PHONY: setup chart chart-lint certs trust-ca untrust-ca caddy-image demo-auth

CERTS_DIR       := .secrets/certs

# TODO: standardize how contributors install pipx; pending prod infra & provider choices.
setup:
	@command -v pipx >/dev/null || { echo "ERROR: pipx not found — install pipx (e.g. 'python3 -m pip install --user pipx') and re-run."; exit 1; }
	@pipx run pre-commit install \
		|| { echo "ERROR: pipx install pre-commit failed — fix and re-run."; exit 1; }
	@echo "==> Wiping all compose state (sledgehammer reset)"
	@echo "    TODO: normally this would split into more granular setup tasks,"
	@echo "    each individually runnable with --force (e.g. per service-"
	@echo "    catalogue item). That's beyond the scope of this take-home —"
	@echo "    fresh state every setup is fine for the demo."
	docker compose down -v
	@echo "==> Regenerating LocalMesh CA + per-service certs + .env + Caddyfile"
	@$(MAKE) certs
	@echo "==> Setup complete. Run 'docker compose up -d --build' to start the stack."

# TODO: detect OS/arch — tools/ binaries hardcoded to linux-amd64; pending prod infra & provider choices.
chart:
	./tools/katenary-3.0.0-rc6-linux-amd64 convert --force

chart-lint:
	./tools/helm-v4.1.4-linux-amd64 lint chart

# LocalMesh dev-machine bootstrap. One button: untrust the old CA, blow
# away .secrets/certs/, regenerate everything via scripts/secrets-gen.py
# (CA + per-service leaf certs + .env managed section + Caddyfile.generated
# + dex.yaml.generated), then install the new CA into the host trust store.
# Always clean-slate; granular subtargets are out of scope for this phase.
#
# See docs/superpowers/specs/2026-05-18-localmesh-service-mesh-design.md §7.
certs: untrust-ca
	@rm -rf $(CERTS_DIR)
	@./scripts/secrets-gen.py
	@$(MAKE) trust-ca

# Install the LocalMesh CA into the host trust store so browsers / curl /
# libraries verify cleanly. Idempotent. Run standalone after a fresh clone
# (with an existing .secrets/certs/ca.crt) or to re-trust without regen.
trust-ca:
	@./tools/step certificate install $(CERTS_DIR)/ca.crt

# Remove the LocalMesh CA from the host trust store. Silent if not present
# so this is safe to chain from `certs` on first run. Useful standalone
# when uninstalling the project.
untrust-ca:
	@./tools/step certificate uninstall $(CERTS_DIR)/ca.crt 2>/dev/null || true

caddy-image:
	docker build -t localmesh/caddy:dev service_catalog/caddy/

demo-auth: setup
	@echo "==> Bringing up the LocalMesh stack with auth at ingress"
	docker compose up -d --build
	@echo
	@echo "==> Stack up. Open https://www.metrics-collector.lvh.me:8443"
	@echo "    First visit will 302 to Dex's built-in login form."
	@echo "    Log in as alice@example.invalid (or bob/charlie) with password 'dev'."
	@echo "==> Verify the JWT-derived row landed in postgres:"
	@echo "    docker compose exec postgres psql -U postgres -c \\"
	@echo "      'SELECT message, jwt_subject FROM hello_messages ORDER BY id DESC LIMIT 3'"

# End ops team responsibility

# Begin individual team responsibility

test:
	@echo "To be done per-project"

# Others could include `lint`, `build`, etc. 

# End individual team responsibility
