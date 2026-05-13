# TODO: convert to multi-stage build (slim base, non-root user) once the
# production base-image policy is settled. See DESIGN_DECISIONS.md
# "Containers" row.
#
# Build context is the repo root (parity with server/Dockerfile so future
# chunks can reach ../proto/ for shared-schema work).
#
# Selective COPY (rather than `COPY www/ ./`) so host-built `.next/` and
# `node_modules/` aren't shipped into the image. Add new source dirs here
# as they land.
FROM node:22-alpine
WORKDIR /app
COPY www/package.json www/package-lock.json ./
RUN npm ci
COPY www/tsconfig.json www/next-env.d.ts ./
COPY www/pages ./pages
RUN npm run build
EXPOSE 3000
CMD ["npm", "start"]
