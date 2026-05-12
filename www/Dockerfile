# TODO: convert to multi-stage build (next standalone output + slim runtime)
# once the production base-image policy is settled. See DESIGN_DECISIONS.md.
#
# Build context is the repo root so proto/ is reachable at runtime by
# @grpc/proto-loader. Compose sets context: . and dockerfile: www/Dockerfile.
FROM node:22-bookworm
WORKDIR /app
COPY proto/ ./proto/
COPY www/package.json www/package-lock.json ./www/
WORKDIR /app/www
RUN npm ci
COPY www/ ./
RUN npm run build
EXPOSE 3000
CMD ["npm", "run", "start"]
