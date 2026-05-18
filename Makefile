# Begin ops team responsibility

.PHONY: setup chart chart-lint certs check-hosts

CERTS_DIR       := .secrets/certs
HOSTS_FILE      ?= /etc/hosts
REQUIRED_HOSTS  ?= www grafana

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
	@echo "==> Bringing up fresh grafana"
	docker compose up -d --wait grafana
	@echo "==> Minting service-account token"
	docker compose run --rm grafana-bootstrap
	@echo "==> Setup complete. Run 'docker compose up -d --build' to start the stack."

# TODO: detect OS/arch — tools/ binaries hardcoded to linux-amd64; pending prod infra & provider choices.
chart:
	./tools/katenary-3.0.0-rc6-linux-amd64 convert --force

chart-lint:
	./tools/helm-v4.1.4-linux-amd64 lint chart

# LocalMesh dev-machine bootstrap. One button: untrust the old CA, blow
# away .secrets/certs/, regenerate everything via scripts/secrets-gen.py
# (CA + per-service leaf certs + .env managed section + Caddyfile.generated),
# then install the new CA into the host trust store. Always clean-slate;
# granular subtargets are out of scope for this phase.
#
# See docs/superpowers/specs/2026-05-18-localmesh-service-mesh-design.md §7.
certs:
	@./tools/step certificate uninstall $(CERTS_DIR)/ca.crt 2>/dev/null || true
	@rm -rf $(CERTS_DIR)
	@./scripts/secrets-gen.py
	@./tools/step certificate install $(CERTS_DIR)/ca.crt

check-hosts:
	@. .env 2>/dev/null && \
	missing=""; \
	for sub in $(REQUIRED_HOSTS); do \
	   fqdn="$$sub.$$PROJECT_NAME.$$LOCAL_DOMAIN"; \
	   if ! grep -qE "127\.0\.0\.1[[:space:]]+$$fqdn" $(HOSTS_FILE); then \
	     missing="$$missing $$fqdn"; \
	   fi; \
	done; \
	if [ -n "$$missing" ]; then \
	   echo "Missing /etc/hosts entries:"; \
	   for h in $$missing; do echo "    127.0.0.1   $$h"; done; \
	   echo "Add the above lines to $(HOSTS_FILE) (requires sudo), then re-run."; \
	   exit 1; \
	fi
# TODO: needs_prod_decisions seed /etc/hosts from project.toml via privileged-init script

# End ops team responsibility

# Begin individual team responsibility

test:
	@echo "To be done per-project"

# Others could include `lint`, `build`, etc. 

# End individual team responsibility
