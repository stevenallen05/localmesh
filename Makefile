# Begin ops team responsibility

.PHONY: setup chart chart-lint

# TODO: standardize how contributors install pipx; pending prod infra & provider choices.
setup:
	@command -v pipx >/dev/null || { echo "ERROR: pipx not found — install pipx (e.g. 'python3 -m pip install --user pipx') and re-run."; exit 1; }
	@pipx run pre-commit install \
		|| { echo "ERROR: pipx install pre-commit failed — fix and re-run."; exit 1; }
	@echo "==> Wiping all compose state (sledgehammer reset)"
	@echo "    TODO: a real dev tool would gate this behind --force and offer a"
	@echo "    granular 'reset just grafana credentials' path. The drift cases"
	@echo "    (persisted admin password, stale tokens) are real but out of"
	@echo "    scope for this take-home — fresh state every setup is fine for"
	@echo "    the demo."
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

# End ops team responsibility

# Begin individual team responsibility

test:
	@echo "To be done per-project"

# Others could include `lint`, `build`, etc. 

# End individual team responsibility
