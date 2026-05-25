# LocalMesh

## The idea

A service mesh is what you get when all production infrastructure for service-to-service communication is treated as one system.

It includes things like routing, service discovery, load balancing, TLS, identity, retries, and observability.

In real companies, this is usually not one product. It’s a combination of Kubernetes configuration, cloud networking, Helm charts, and SRE-managed infrastructure.

LocalMesh makes that system portable.

It lets you run a production-shaped service mesh locally, then compile the same setup into Helm charts for deployment.

---

## What LocalMesh does

LocalMesh gives you:

* A local runtime that behaves like production infrastructure
* A plugin system for common dependencies (databases, caches, etc.)
* A compiler from local environment → Helm charts

You don’t maintain separate dev and prod infrastructure. You define one system and run it in both places.

---

## Install

```bash
curl -sSL https://localmesh.dev/install | bash
```

or:

```bash
go install github.com/stevenallen05/localmesh@latest
```

Verify:

```bash
localmesh version
```

---

## Quick start

Create a project:

```bash
localmesh init my-app
cd my-app
```

Start the local mesh:

```bash
localmesh up
```

At this point, you have a running infrastructure shell. Services and plugins plug into it.

---

## Add a plugin (Redis)

Plugins are infrastructure dependencies that work in both local and production environments.

Add Redis:

```bash
localmesh plugin add redis
```

Now Redis is available inside the mesh at:

```
redis:6379
```

No host configuration. No environment switching.

---

## Add a service

```bash
localmesh service add api
```

Example dependency wiring in `mesh.yaml`:

```yaml
services:
  api:
    depends_on:
      - redis
```

---

## Run everything

```bash
localmesh up
```

You now have:

* API service running locally
* Redis running as a plugin
* Service-to-service networking handled by the mesh

---

## Build for production

```bash
localmesh build
```

This generates Helm charts:

```
dist/helm/
```

Deploy with:

```bash
helm install my-app ./dist/helm
```

The same system that ran locally is now deployed in Kubernetes.

---

## Mental model

LocalMesh treats infrastructure as one system:

* services
* plugins
* networking
* identity
* routing

It runs locally as a development environment, and compiles into production infrastructure using Helm.

---

## Plugins

A plugin is a packaged infrastructure dependency.

It defines:

* how it runs locally (Docker-based)
* how it runs in production (Helm-based)

Example: Redis behaves the same from the service’s perspective in both environments.

---

## Glossary

service mesh
A unified system for service-to-service communication in distributed systems, including routing, discovery, load balancing, TLS, identity, retries, and observability.

dataplane
The runtime layer that handles traffic between services.

control plane
The system that configures routing, identity, and policies across services.

plugin
A reusable infrastructure module that works in both local and production environments.

