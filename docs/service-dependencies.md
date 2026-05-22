# Local Dev Service Dependencies

Each section gives a `docker-compose.yaml` snippet for a common service dependency. The compose service uses a standard local-dev image; the `katenary.v3/dependencies` label points at the Helm chart that will be wired in when Katenary converts this compose file for production. Three alternates are listed per service — the first is active, the others are commented out. Swap them by commenting the active block and uncommenting another.

> The `katenary.v3/dependencies` value mirrors a Helm `Chart.yaml` dependency entry (`name`, `version`, `repository`). Verify field names against `katenary help-label dependencies` for the installed version.

## Postgres

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: app
      POSTGRES_PASSWORD: app
      POSTGRES_DB: app
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    labels:
      # Self-hosted CNCF Postgres operator cluster
      katenary.v3/dependencies: |-
        - name: cluster
          version: 0.x.x
          repository: https://cloudnative-pg.github.io/charts

      # Crunchy Data commercial Postgres operator
      # katenary.v3/dependencies: |-
      #   - name: pgo
      #     version: 5.x.x
      #     repository: oci://registry.developers.crunchydata.com/crunchydata

      # AWS managed RDS/Aurora via ACK
      # katenary.v3/dependencies: |-
      #   - name: rds-chart
      #     version: 1.x.x
      #     repository: oci://public.ecr.aws/aws-controllers-k8s

volumes:
  postgres_data:
```

## Redis

```yaml
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    labels:
      # Redis Inc commercial self-hosted operator
      katenary.v3/dependencies: |-
        - name: redis-enterprise-operator
          version: 8.x.x
          repository: https://helm.redis.io

      # Open-source clustered Redis operator (OT)
      # katenary.v3/dependencies: |-
      #   - name: redis-operator
      #     version: 0.x.x
      #     repository: https://ot-container-kit.github.io/helm-charts

      # AWS managed ElastiCache via ACK
      # katenary.v3/dependencies: |-
      #   - name: elasticache-chart
      #     version: 1.x.x
      #     repository: oci://public.ecr.aws/aws-controllers-k8s

volumes:
  redis_data:
```

## File storage (S3-compatible object storage)

```yaml
services:
  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"   # S3 API
      - "9001:9001"   # web console
    volumes:
      - minio_data:/data
    labels:
      # Self-hosted S3-compatible MinIO tenant
      katenary.v3/dependencies: |-
        - name: tenant
          version: 5.x.x
          repository: https://operator.min.io

      # Self-hosted Ceph S3 gateway (Rook)
      # katenary.v3/dependencies: |-
      #   - name: rook-ceph-cluster
      #     version: 1.x.x
      #     repository: https://charts.rook.io/release

      # AWS managed S3 buckets via ACK
      # katenary.v3/dependencies: |-
      #   - name: s3-chart
      #     version: 1.x.x
      #     repository: oci://public.ecr.aws/aws-controllers-k8s

volumes:
  minio_data:
```
