# Begin ops team responsibility

.PHONY: setup chart chart-lint

# TODO: standardize how contributors install pipx; pending prod infra & provider choices.
setup:
	@command -v pipx >/dev/null || { echo "ERROR: pipx not found — install pipx (e.g. 'python3 -m pip install --user pipx') and re-run."; exit 1; }
	pipx run pre-commit install \
		|| { echo "ERROR: pipx install pre-commit failed — fix and re-run."; exit 1; }
	@echo "==> Ensuring grafana is up (idempotent)"
	docker compose up -d --wait grafana
	@echo "==> Forcing admin password to 'admin' (overrides any prior state)"
	docker compose exec -T grafana grafana-cli admin reset-admin-password admin
	@echo "==> Minting service-account token (overwrites any previous token)"
	docker compose run --rm grafana-bootstrap
	@echo "==> Restarting server so it picks up the new token (if running)"
	docker compose restart server 2>/dev/null || true
	@echo "==> Setup complete. Run 'docker compose up -d' to start the stack."

# TODO: detect OS/arch — tools/ binaries hardcoded to linux-amd64; pending prod infra & provider choices.
chart:
	./tools/katenary-3.0.0-rc6-linux-amd64 convert --force

chart-lint:
	./tools/helm-v4.1.4-linux-amd64 lint chart

# End ops team responsibility

# Begin individual team responsibility

test:
 echo "To be done per-project"

# Others could include `lint`, `build`, etc. 

# End individual team responsibility
