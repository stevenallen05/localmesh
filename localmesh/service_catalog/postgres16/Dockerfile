# Build pg_tracing against postgres 16. The extension publishes Tempo-compatible
# spans for parse / plan / execute stages and pushes them OTLP/HTTP. pg_tracing
# does not yet support PG17, which is why we pin to 16 here.
FROM postgres:16-bookworm

ARG PG_TRACING_REF=v0.1.3

RUN set -eux; \
    apt-get update; \
    # libcurl4 stays after the purge — pg_tracing's OTLP push links it at runtime.
    apt-get install -y --no-install-recommends \
        ca-certificates \
        git \
        build-essential \
        libcurl4-openssl-dev \
        postgresql-server-dev-16; \
    git clone --depth=1 --branch "${PG_TRACING_REF}" \
        https://github.com/DataDog/pg_tracing.git /tmp/pg_tracing; \
    cd /tmp/pg_tracing; \
    make USE_PGXS=1 install; \
    cd /; \
    rm -rf /tmp/pg_tracing; \
    # Toolchain is build-only — strip; keep libcurl4 (runtime dep).
    apt-get purge -y --auto-remove \
        git build-essential libcurl4-openssl-dev postgresql-server-dev-16; \
    apt-get install -y --no-install-recommends libcurl4; \
    rm -rf /var/lib/apt/lists/*
