.PHONY: setup chart chart-lint env

# TODO: standardize how contributors install pipx; pending prod infra & provider choices.
setup: env
	@command -v pipx >/dev/null || { echo "ERROR: pipx not found — install pipx (e.g. 'python3 -m pip install --user pipx') and re-run."; exit 1; }
	pipx run pre-commit install \
		--hook-type pre-commit \
		--hook-type post-checkout \
		--hook-type post-merge \
		--hook-type post-rewrite \
		|| { echo "ERROR: pipx install pre-commit failed — fix and re-run."; exit 1; }

# Writes .env with IMAGE_TAG derived from the current git branch. The
# post-checkout / post-merge / post-rewrite pre-commit hooks keep this in sync
# after `make setup`; this target is the bootstrap.
env:
	@printf '# Auto-managed by pre-commit; do not edit.\nIMAGE_TAG=%s\n' "$$(git rev-parse --abbrev-ref HEAD | tr / -)" > .env

# TODO: detect OS/arch — tools/ binaries hardcoded to linux-amd64; pending prod infra & provider choices.
chart: env
	set -a && . ./.env && set +a && ./tools/katenary-3.0.0-rc6-linux-amd64 convert --force

chart-lint:
	./tools/helm-v4.1.4-linux-amd64 lint chart
