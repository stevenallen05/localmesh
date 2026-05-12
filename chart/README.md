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

| Parameter                 | Default                 |
| ------------------------- | ----------------------- |
| `agent.imagePullPolicy`   | `IfNotPresent`          |
| `agent.replicas`          | `1`                     |
| `agent.repository.image`  | ``                      |
| `agent.repository.tag`    | ``                      |
| `agent.serviceAccount`    | ``                      |
| `db.imagePullPolicy`      | `IfNotPresent`          |
| `db.replicas`             | `1`                     |
| `db.repository.image`     | `timescale/timescaledb` |
| `db.repository.tag`       | `2.18.0-pg17`           |
| `db.serviceAccount`       | ``                      |
| `server.imagePullPolicy`  | `IfNotPresent`          |
| `server.replicas`         | `1`                     |
| `server.repository.image` | ``                      |
| `server.repository.tag`   | ``                      |
| `server.serviceAccount`   | ``                      |
| `www.imagePullPolicy`     | `IfNotPresent`          |
| `www.replicas`            | `1`                     |
| `www.repository.image`    | ``                      |
| `www.repository.tag`      | ``                      |
| `www.serviceAccount`      | ``                      |


