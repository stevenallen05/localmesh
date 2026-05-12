.PHONY: setup chart chart-lint

# TODO: standardize how contributors install pipx; pending prod infra & provider choices.
setup:
	@command -v pipx >/dev/null || { echo "ERROR: pipx not found — install pipx (e.g. 'python3 -m pip install --user pipx') and re-run."; exit 1; }
	pipx run pre-commit install \
		|| { echo "ERROR: pipx install pre-commit failed — fix and re-run."; exit 1; }

# TODO: detect OS/arch — tools/ binaries hardcoded to linux-amd64; pending prod infra & provider choices.
chart:
	./tools/katenary-3.0.0-rc6-linux-amd64 convert --force

chart-lint:
	./tools/helm-v4.1.4-linux-amd64 lint chart
