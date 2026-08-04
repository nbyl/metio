# Metio - Minecraft Server Control Panel

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![CI](https://github.com/nbyl/metio/actions/workflows/ci.yml/badge.svg)](https://github.com/nbyl/metio/actions)

A self-hosted Minecraft server management platform running on Google Cloud Platform. Start, stop, and manage Minecraft servers on-demand through a web interface — pay only for what you use.

Metio provisions Compute Engine VMs on the fly, so your server runs only when players are online. No more paying for 24/7 idle servers.

## Features

- **On-demand servers** — Start and stop your Minecraft server from a web UI. VMs auto-provision and tear down.
- **Cost control** — Servers only run when needed. Scheduled shutdowns prevent runaway costs.
- **One-click setup** — OAuth login, initial setup wizard, and server creation in under 5 minutes.
- **Whitelist management** — Add/remove players via the UI. Synced to the server automatically.
- **Player dashboard** — See who's online, server version, uptime, and performance at a glance.
- **Server profiles** — Configure CPU, memory, region, and Minecraft version per server.
- **Backup support** — Attach persistent storage for world backups.
- **Multi-server** — Run multiple independent Minecraft servers from a single deployment.

## Quick Start

### Prerequisites

- A Google Cloud Platform project (with billing enabled)
- A domain or subdomain pointing to your deployment
- A Google OAuth 2.0 client ID and secret

### Deploy

```bash
# Clone the repository
git clone https://github.com/nbyl/metio
cd metio

# Deploy the full stack (requires GCP project setup)
make deploy
```

Follow the [Deployment Guide](docs/DEPLOYMENT.md) for a complete walkthrough from fresh GCP project to first Minecraft server.

### First Server

Once deployed, visit your Metio instance, complete the setup wizard, and create your first server. Metio will provision a Compute Engine VM, install Minecraft, and make it available — usually within 2-3 minutes.

## Documentation

| Guide | Description |
|-------|-------------|
| [Deployment Guide](docs/DEPLOYMENT.md) | Full GCP project setup through production deployment |
| [Contributing](CONTRIBUTING.md) | Development setup, coding standards, and PR process |
| [Code of Conduct](CODE_OF_CONDUCT.md) | Community guidelines and reporting |
| [Security Policy](SECURITY.md) | Reporting vulnerabilities and supported versions |

## Architecture

```
┌─────────────┐      ┌────────────────────────┐
│   Browser   │──────│   Cloud Run           │
│  (React UI) │      │  (Controller + daprd) │────┐
└─────────────┘      └──────────┬─────────────┘    │ Dapr state store
                               │                    │ (Datastore-mode)
                               │ Pub/Sub            │
                               ▼                    │
                      ┌─────────────────┐           │
                      │ Compute Engine  │───────────┘
                      │ (Minecraft +    │
                      │  Machine Agent) │
                      └─────────────────┘
```

- **Browser**: React SPA served by Cloud Run
- **Controller**: Go backend handling API requests, OAuth, and Pulumi orchestration
- **Dapr state store**: Stores server state, player counts, configuration via the Dapr sidecar
- **Machine Agent**: Runs on each VM, reports Minecraft status through the controller API
- **Pub/Sub**: Notifies controller of VM lifecycle events

## Tech Stack

### Frontend
- React 19, TypeScript, Vite
- TailwindCSS 4, Lucide React icons
- React Query (server state), React Router

### Backend
- Go 1.25, Gorilla Mux, Viper
- Google Cloud client libraries
- Pulumi Automation API for infrastructure orchestration

### Infrastructure
- GCP: Cloud Run, Compute Engine, Firestore (Dapr state store), Pub/Sub
- CI/CD: GitHub Actions, OpenTofu
- Container registry: ghcr.io

## Development Setup

### Prerequisites

- Go 1.25+
- Node.js 20+ (see `.nvmrc`)
- gcloud CLI (authenticated)
- Docker (for image builds)
- OpenTofu (for infrastructure deployment)

### Initial Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/nbyl/metio
   cd metio
   ```

2. Install frontend dependencies:
   ```bash
   cd web && npm ci
   ```

3. Configure GCP authentication:
   ```bash
   gcloud auth application-default login
   ```

4. Set up environment variables (create `.env` or export):
   ```bash
   export GCP_PROJECT="your-project-id"
   export GCP_ZONE="europe-west3-a"
   export REGION="europe-west3"
   export ENVIRONMENT="development"
   export INSTANCE_NAME="your-minecraft-server"
   export SESSION_KEY="your-secret-session-key"
   export BASE_URL="http://localhost:8080"
   export GOOGLE_CLIENT_ID="your-oauth-client-id"
   export GOOGLE_CLIENT_SECRET="your-oauth-client-secret"
   export ALLOWED_USERS="your-email@example.com"
   ```

### Running Locally

**Option 1: Full-stack development (recommended)**

```bash
# Terminal 1: Frontend dev server with hot reload
cd web && npm run dev

# Terminal 2: Go backend with auto-reload
air
```

- Frontend runs on http://localhost:5173
- Backend runs on http://localhost:8080
- Vite proxies `/api` and `/auth` requests to the backend automatically

**Option 2: Backend only (uses embedded frontend)**

```bash
DEV_MODE=true air
```

This serves the pre-built frontend from `static/dist/`. Run `cd web && npm run build` first.

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | No | Server port (default: 8080) |
| `GCP_PROJECT` | Yes | GCP project ID |
| `GCP_ZONE` | Yes | GCP zone (e.g., `europe-west3-a`) |
| `REGION` | Yes | GCP region (e.g., `europe-west3`) |
| `INSTANCE_NAME` | Yes | Compute Engine instance name |
| `ENVIRONMENT` | Yes | Environment name (`development`/`production`) |
| `SESSION_KEY` | Yes | Secret key for session cookies |
| `BASE_URL` | Yes | Application base URL for OAuth redirects |
| `GOOGLE_CLIENT_ID` | Yes | Google OAuth 2.0 client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | Google OAuth 2.0 client secret |
| `ALLOWED_USERS` | Yes | Comma-separated list of allowed email addresses |
| `DEV_MODE` | No | Set to `true` to serve frontend from filesystem |

## Building

```bash
# Build all binaries (includes frontend build)
make

# Build specific binary
make controller
make machine-agent

# Build Docker images (local, pushes to ghcr.io)
make controller-image
make machine-agent-image
make push-images

# Promote image tag (retag for deployment)
make promote FROM=a1b2c3d4 TO=main
```

## Testing

```bash
# Run all Go tests with coverage
make test
# Coverage report: build/coverage.html

# Run frontend tests
cd web && npm run test:run

# Run frontend tests with coverage
cd web && npm run test:coverage
# Coverage report: web/coverage/
```

## Deployment

For a complete guide covering GCP project setup through first server creation, see [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md).

```bash
# Full deployment (build all images + deploy infrastructure)
make deploy

# Deploy controller only (faster iteration for UI changes)
make deploy-controller

# Deploy machine-agent only (triggers VM recreation)
make deploy-machine-agent

# Deploy infrastructure without rebuilding images
make deploy-infrastructure
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for:

- Development setup and workflow
- Coding standards and commit conventions
- Pull request process
- Testing guidelines

This project adheres to a [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold its terms.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
