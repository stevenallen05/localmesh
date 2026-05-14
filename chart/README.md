# challenge

A Helm chart for challenge

## Installing the Chart

To install the chart with the release name `my-release`:

```bash
# Standard Helm install
$ helm install  my-release challenge

# To use a custom namespace and force the creation of the namespace
$ helm install my-release --namespace my-namespace --create-namespace challenge

# To use a custom values file
$ helm install my-release -f my-values.yaml challenge
```

See the [Helm documentation](https://helm.sh/docs/intro/using_helm/) for more information on installing and managing the chart.

## Configuration

The following table lists the configurable parameters of the challenge chart and their default values.

| Parameter                                                 | Default                                |
| --------------------------------------------------------- | -------------------------------------- |
| `cadvisor.imagePullPolicy`                                | `IfNotPresent`                         |
| `cadvisor.replicas`                                       | `1`                                    |
| `cadvisor.repository.image`                               | `gcr.io/cadvisor/cadvisor`             |
| `cadvisor.repository.tag`                                 | `v0.52.1`                              |
| `cadvisor.serviceAccount`                                 | ``                                     |
| `grafana.imagePullPolicy`                                 | `IfNotPresent`                         |
| `grafana.persistence.grafana_data.accessMode[0].value`    | `ReadWriteOnce`                        |
| `grafana.persistence.grafana_data.enabled`                | `true`                                 |
| `grafana.persistence.grafana_data.size`                   | `1Gi`                                  |
| `grafana.persistence.grafana_data.storageClass`           | `-`                                    |
| `grafana.replicas`                                        | `1`                                    |
| `grafana.repository.image`                                | `grafana/grafana`                      |
| `grafana.repository.tag`                                  | `11.4.0`                               |
| `grafana.serviceAccount`                                  | ``                                     |
| `node_exporter.imagePullPolicy`                           | `IfNotPresent`                         |
| `node_exporter.replicas`                                  | `1`                                    |
| `node_exporter.repository.image`                          | `prom/node-exporter`                   |
| `node_exporter.repository.tag`                            | `v1.8.2`                               |
| `node_exporter.serviceAccount`                            | ``                                     |
| `otel_collector.imagePullPolicy`                          | `IfNotPresent`                         |
| `otel_collector.replicas`                                 | `1`                                    |
| `otel_collector.repository.image`                         | `otel/opentelemetry-collector-contrib` |
| `otel_collector.repository.tag`                           | `0.152.0`                              |
| `otel_collector.serviceAccount`                           | ``                                     |
| `redis.imagePullPolicy`                                   | `IfNotPresent`                         |
| `redis.persistence.redis_data.accessMode[0].value`        | `ReadWriteOnce`                        |
| `redis.persistence.redis_data.enabled`                    | `true`                                 |
| `redis.persistence.redis_data.size`                       | `1Gi`                                  |
| `redis.persistence.redis_data.storageClass`               | `-`                                    |
| `redis.replicas`                                          | `1`                                    |
| `redis.repository.image`                                  | `redis`                                |
| `redis.repository.tag`                                    | `7.4-alpine`                           |
| `redis.serviceAccount`                                    | ``                                     |
| `server.imagePullPolicy`                                  | `IfNotPresent`                         |
| `server.persistence.grafana_token.accessMode[0].value`    | `ReadWriteOnce`                        |
| `server.persistence.grafana_token.enabled`                | `true`                                 |
| `server.persistence.grafana_token.size`                   | `1Gi`                                  |
| `server.persistence.grafana_token.storageClass`           | `-`                                    |
| `server.replicas`                                         | `1`                                    |
| `server.repository.image`                                 | `metrics-stack/server`                 |
| `server.repository.tag`                                   | ``                                     |
| `server.serviceAccount`                                   | ``                                     |
| `tempo.imagePullPolicy`                                   | `IfNotPresent`                         |
| `tempo.persistence.tempo_data.accessMode[0].value`        | `ReadWriteOnce`                        |
| `tempo.persistence.tempo_data.enabled`                    | `true`                                 |
| `tempo.persistence.tempo_data.size`                       | `1Gi`                                  |
| `tempo.persistence.tempo_data.storageClass`               | `-`                                    |
| `tempo.replicas`                                          | `1`                                    |
| `tempo.repository.image`                                  | `grafana/tempo`                        |
| `tempo.repository.tag`                                    | `2.10.5`                               |
| `tempo.serviceAccount`                                    | ``                                     |
| `victoriametrics.imagePullPolicy`                         | `IfNotPresent`                         |
| `victoriametrics.persistence.vm_data.accessMode[0].value` | `ReadWriteOnce`                        |
| `victoriametrics.persistence.vm_data.enabled`             | `true`                                 |
| `victoriametrics.persistence.vm_data.size`                | `1Gi`                                  |
| `victoriametrics.persistence.vm_data.storageClass`        | `-`                                    |
| `victoriametrics.replicas`                                | `1`                                    |
| `victoriametrics.repository.image`                        | `victoriametrics/victoria-metrics`     |
| `victoriametrics.repository.tag`                          | `v1.106.0`                             |
| `victoriametrics.serviceAccount`                          | ``                                     |
| `www.imagePullPolicy`                                     | `IfNotPresent`                         |
| `www.ingress.class`                                       | `-`                                    |
| `www.ingress.enabled`                                     | `false`                                |
| `www.ingress.host`                                        | `www.tld`                              |
| `www.ingress.path`                                        | `/`                                    |
| `www.ingress.tls.enabled`                                 | `true`                                 |
| `www.ingress.tls.secretName`                              | ``                                     |
| `www.replicas`                                            | `1`                                    |
| `www.repository.image`                                    | `metrics-stack/www`                    |
| `www.repository.tag`                                      | `0.1.0`                                |
| `www.serviceAccount`                                      | ``                                     |


