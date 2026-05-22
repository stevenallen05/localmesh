# Begin ops team responsibility

.PHONY: setup chart chart-lint demo-auth

setup:
	@echo "==> Wiping all compose state (sledgehammer reset)"
	docker compose down -v
	@go run ./localmesh_src/cmd/localmesh setup
	@echo "==> Setup complete. Run 'docker compose up -d --build' to start the stack."

# TODO: detect OS/arch — localmesh/bin/ binaries hardcoded to linux-amd64; pending prod infra & provider choices.
chart:
	./localmesh/bin/katernary convert --force

chart-lint:
	./localmesh/bin/helm lint chart

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
