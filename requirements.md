# 📊 Metrics/Latency Monitoring System - Take-Home Technical Test

This project is a **take-home technical test** that demonstrates a distributed metrics monitoring system. The system consists of three main components:

1. **Metrics Agent** - Containerized client that measures metrics (latency, disk I/O, etc.) and submits them to the server
2. **Server** - A Rust backend server that stores the metrics data
3. **Web Application** - A Next.js frontend that displays the metrics retrieved from the server

The system uses Protocol Buffers / gRPC as an RPC transport framework. There is already a proof of concept gRPC connection between the Next.js Web Application and the Rust Server.

## 🏗️ Architecture

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│   Metrics   │─────▶│   Server    │◀─────│   Next.js   │
│    Agent    │      │   (Rust)    │      │  Frontend   │
│             │      │             │      │             │
│ Measures    │      │ Stores      │      │ Displays    │
│ e.g. Latency│      │ Metrics     │      │ Metrics     │
└─────────────┘      └─────────────┘      └─────────────┘
```

### Component Details

- **Metrics Agent**: MUST be containerized. Can be any language (Rust, Python, Go, etc.).
- **Server**: Rust backend. Stores metrics (in-memory, file, or database). Containerization optional.
- **Web Application**: Next.js frontend. Displays metrics with good formatting/visualization. Containerization optional.

## 📁 Project Structure

```
.
├── agent/           # Create this yourself
├── proto/           # Protocol Buffer definitions for gRPC
├── server/          # Rust backend server
└── www/             # Next.js frontend client
```

## Getting Started

**Rust Backend Server:**

```bash
cd server
cargo run
```

**Next.js Web Application:**

```bash
cd www
npm install
npm run dev
```

## ✅ Tasks

- Create a python client called Metrics Agent
- Create a system to deploy the Metrics Agent (client) to a container.
  - The Server and Web Application can optionally be containerized.
- The overall system should measure and track metrics from the Metrics Agent, store them on the server, and display them in the Web Application.

### Core Requirements

- ✅ **gRPC MUST be used in your final solution.**
  - There is already a proof of concept gRPC connection between the Next.js Web Application and the Rust Server.
- ✅ **Metrics Agent MUST be containerized** (Docker, Rancher Desktop, Podman, Nix, etc.) 📦
  - Include explanation of your container solution choice
  - Server and Web Application containerization is optional
- ✅ **You CAN use a container orchestration tool (Docker Swarm, Kubernetes, Rancher, etc...) to help manage the containers.** 🌐

- ✅ **Metrics Agent MUST measure and submit metrics to the server** ⏱️
  - Be creative! Metric examples may include disk latency, network latency, cpu usage, memory usage, or some other numerical point-in-time system performance metric(s).
  - Can be implemented in any language (Rust, Python, Go, etc.)
- ✅ **Server MUST store metrics** (in-memory, file storage, or database) 💾
- ✅ **Web Application MUST retrieve and display/visualize metrics** 📈

### Requirements

- ✅ **Publish this as a GitHub repository** 📦
- ✅ **Clear README with run instructions** 📖
  - Tip: Kubernetes manifests, Helm charts, or Docker Compose, etc. will reduce command-line steps
- ✅ **Should be able to run in GitHub Codespaces simply by following the steps in your README** 🚀

### Bonus Points

- ✅ **Good error handling** 🛡️
- ✅ **Live demo link** 🔗
- ✅ **Ability to adjust polling/submission intervals remotely without restarting the Agent** ⚡
- ✅ **Horizontal scaling**
  - ability to deploy multiple Metrics Agent and/or server/web application instances as needed
- ✅ **Good database design**
  - with explanation of your choice of database and design considerations
- ✅ **Good Logging** (requests, responses, errors, etc.)
