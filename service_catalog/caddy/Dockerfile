# Multi-stage Caddy build: stage 1 compiles Caddy with the local
# enduser_attrs module (which stamps OTel span attributes from
# request headers — see plugin/enduser_attrs/module.go); stage 2
# ships the static binary on alpine.
#
# TODO: needs_prod_decisions Caddy build provenance + image signing

ARG CADDY_VERSION=2.8.4

FROM caddy:${CADDY_VERSION}-builder-alpine AS builder
ARG CADDY_VERSION

COPY plugin/enduser_attrs /src/enduser_attrs

# --with <module>=<local-path> overrides the module resolution to use
# the local source instead of fetching from github. The module's go.mod
# pins its caddy + otel deps; xcaddy aligns them with the build version.
RUN xcaddy build "v${CADDY_VERSION}" \
    --with "github.com/localmesh/caddy-enduser-attrs=/src/enduser_attrs"

FROM alpine:3.20
RUN apk add --no-cache ca-certificates mailcap wget \
 && addgroup -S caddy -g 1000 \
 && adduser -S -D -H -h /var/lib/caddy -s /sbin/nologin -G caddy -u 1000 caddy

COPY --from=builder /usr/bin/caddy /usr/bin/caddy

# Mirror upstream caddy:alpine's defaults so the rest of the project
# (volumes, command, healthcheck) doesn't have to know it's a custom build.
# Pre-chown /config + /data so the non-root caddy user can write its
# autosave + storage-clean lock files (anonymous volumes inherit the
# underlying directory's ownership at first mount).
ENV XDG_CONFIG_HOME=/config XDG_DATA_HOME=/data
RUN mkdir -p /config /data && chown caddy:caddy /config /data
VOLUME /config /data
EXPOSE 8443 2019
WORKDIR /srv
USER caddy

ENTRYPOINT ["caddy"]
CMD ["run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
