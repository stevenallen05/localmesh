# TODO: convert to multi-stage build targeting a distroless/non-root base
# once the production base-image policy is settled. See DESIGN_DECISIONS.md.
#
# Build context is the repo root (so ../proto/ is reachable for tonic-build).
# Compose sets context: . and dockerfile: server/Dockerfile.
FROM rust:1.90-bookworm
RUN apt-get update \
    && apt-get install -y --no-install-recommends protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /build
COPY proto/ ./proto/
COPY server/ ./server/
WORKDIR /build/server
RUN cargo build --release
ENTRYPOINT ["/build/server/target/release/server"]
