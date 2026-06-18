# Metio - Minecraft Server Control Panel

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

A self-hosted Minecraft server management application running on Google Cloud Platform. Start and stop your Minecraft server on-demand through a web interface.

## Architecture

```
┌─────────────┐      ┌─────────────────┐      ┌─────────────┐
│   Browser   │──────│   Cloud Run     │──────│  Firestore  │
│  (React UI) │      │  (Controller)   │      │   (State)   │
└─────────────┘      └─────────────────┘      └──────┬──────┘
                              │                       │
                              │ Pub/Sub               │
                              ▼                       │
                     ┌─────────────────┐              │
                     │ Compute Engine  │──────────────┘
                     │ (Minecraft +    │
                     │  Machine Agent) │
                     └─────────────────┘
```

- **Browser**: React SPA served by Cloud Run
- **Controller**: Go backend handling API requests and OAuth
- **Firestore**: Stores server state, player counts, uptime
- **Machine Agent**: Runs on VM, reports Minecraft status to Firestore
- **Pub/Sub**: Notifies controller of VM lifecycle events

## Tech Stack

### Frontend
- React 19, TypeScript, Vite
- TailwindCSS 4, Lucide React icons
- React Query (server state), React Router

### Backend
- Go 1.25, Gorilla Mux, Viper
- Google Cloud client libraries

### Infrastructure
- GCP: Cloud Run, Compute Engine, Firestore, Pub/Sub
- CI/CD: GitHub Actions, OpenTofu

## Prerequisites

- Go 1.25+
- Node.js 20+ (see `.nvmrc`)
- gcloud CLI (authenticated)
- Docker (for image builds)
- OpenTofu (for infrastructure deployment)

## Development Setup

### Initial Setup

1. Clone the repository:
   ```bash
   git clone <repository-url>
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

## Environment Variables

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

# Legacy: build via gcloud builds
make build-images
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

# Run frontend tests in watch mode
cd web && npm run test
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

## Infrastructure Setup

### Initial GCP Setup

1. Create a GCS bucket for OpenTofu state:
   ```bash
   gsutil mb -l <region> gs://<your-project>-tofu-state
   ```

2. Create Artifact Registry repository:
   ```bash
   gcloud artifacts repositories create metio \
       --repository-format=docker \
       --location=<region> \
       --immutable-tags
   ```

3. Initialize and apply infrastructure:
   ```bash
   cd deploy
   tofu init
   tofu apply
   ```

### CI/CD

CI is handled by GitHub Actions (`.github/workflows/ci.yml`):

- Runs tests (Go + frontend) on every push and pull request to `main`
- On `main` merges: builds and pushes Docker images to `ghcr.io/nbyl/metio`
- Images are tagged with the git SHA

### Image Cleanup Policy

GitHub Container Registry automatically deletes non-main branch images older than 3 days. Configure retention in repository **Settings → Packages → Package retention**:

## Troubleshooting

| Issue | Solution |
|-------|----------|
| CORS errors in development | Ensure Vite dev server is running (`npm run dev`) and proxying to backend |
| "Not authenticated" errors | Check `ALLOWED_USERS` includes your email address |
| Firebase/OAuth not working | Verify `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` are correct |
| Frontend changes not showing | Run `cd web && npm run build` or use the Vite dev server |
| VM not starting | Check GCP quotas and Compute Engine API is enabled |

## Project Structure

```
metio/
├── cmd/
│   ├── controller/      # Web server (Cloud Run)
│   └── machine-agent/   # VM status reporter
├── web/            # React SPA (Vite)
│   ├── src/
│   │   ├── components/  # React components
│   │   ├── hooks/       # Custom hooks (React Query)
│   │   └── types/       # TypeScript types
│   └── package.json
├── internal/
│   ├── handlers/        # HTTP handlers
│   ├── db/              # Firestore client
│   ├── services/        # Business logic
│   ├── config/          # Configuration
│   ├── tracing/         # OpenTelemetry
│   └── testutil/        # Test helpers
├── deploy/               # OpenTofu infrastructure
├── static/dist/         # Built frontend (embedded in binary)
└── Makefile
```

## Links

- [Minecraft Server on GCP Guide](https://cloud.google.com/blog/products/management-tools/brick-by-brick-learn-gcp-by-setting-up-a-minecraft-server)
