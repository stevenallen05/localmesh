# TODO: convert to multi-stage build targeting a distroless/non-root base
# once the production base-image policy is settled. See DESIGN_DECISIONS.md.
#
# Build context is the repo root so proto/ is reachable, matching server/www.
# Compose sets context: . and dockerfile: agent/Dockerfile.
FROM golang:1.23-bookworm
WORKDIR /build
COPY proto/ ./proto/
COPY agent/go.mod agent/go.sum ./agent/
WORKDIR /build/agent
RUN go mod download
COPY agent/ ./
RUN go build -o /usr/local/bin/agent .
ENV AGENT_CONFIG_FILE=/build/agent/agent.toml
ENTRYPOINT ["/usr/local/bin/agent"]
