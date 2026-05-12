# TODO: convert to multi-stage build targeting a distroless/non-root base
# once the production base-image policy is settled. See DESIGN_DECISIONS.md.
FROM golang:1.23-bookworm
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /usr/local/bin/agent .
ENV AGENT_CONFIG_FILE=/app/agent.toml
ENTRYPOINT ["/usr/local/bin/agent"]
