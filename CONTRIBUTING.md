# Contributing to Metio

## Development Setup

### Required Tools

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ (see `go.mod`) | Backend |
| Node.js | 25 (see `.nvmrc`) | Frontend |
| [air](https://github.com/air-verse/air) | Latest | Go hot reload |
| gcloud CLI | Latest | GCP auth and API calls |
| Docker | Latest | Image builds |
| OpenTofu | 1.9+ | Infrastructure deployment |

### Clone and Setup

```bash
git clone <repository-url>
cd metio

# Install frontend dependencies
cd web && npm ci && cd ..

# Create dev env file
cp .devcontainer/devcontainer.env.example build/local.env
# Edit build/local.env with your GCP project and OAuth credentials
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GCP_PROJECT` | Yes | GCP project ID |
| `REGION` | Yes | GCP region (e.g., `europe-west3`) |
| `GCP_ZONE` | Yes | GCP zone (e.g., `europe-west3-a`) |
| `ENVIRONMENT` | Yes | Environment name (`development`) |
| `MACHINE_AGENT_IMAGE` | Yes | Docker image for machine agents |
| `SESSION_KEY` | Yes | Secret key for cookie sessions |
| `BASE_URL` | Yes | OAuth callback URL (`http://localhost:8080`) |
| `GOOGLE_CLIENT_ID` | Yes | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | Google OAuth client secret |
| `ALLOWED_USERS` | No | Comma-separated allowed emails |
| `DEV_MODE` | No | Set to `true` to serve frontend from filesystem |
| `DEV_API_KEY` | No | Bypass OAuth in development |

### Running Locally

**Full-stack development (recommended):**

```bash
make develop
```

This starts:
- **Frontend** on `http://localhost:5173` (Vite with hot reload)
- **Backend** on `http://localhost:8080` (air with auto-reload)
- Vite proxies `/api` and `/auth` to the Go backend

**Backend only:**

```bash
# Build frontend first, then serve embedded
cd web && npm run build && cd ..
DEV_MODE=true MACHINE_AGENT_IMAGE=placeholder air
```

### Devcontainer

A development container is configured in `.devcontainer/` for VS Code with:
- All required tools pre-installed (Go, Node, air, gcloud, tofu, opencode)
- VS Code extensions: Go, OpenTofu, GitHub Pull Requests, OpenCode
- Docker bind-mount for container builds
- OpenCode config mount for AI tooling

## Architecture Overview

### System Diagram

```
┌─────────────┐      ┌──────────────────────┐      ┌─────────────┐
│   Browser   │──────│   Cloud Run           │──────│  Firestore  │
│  (React UI) │  │  │  (Controller + API)   │  │  │   (State)   │
└─────────────┘      └──────────────────────┘      └──────┬──────┘
                            │                               │
                            │ Pub/Sub                       │
                            ▼                               │
                      ┌──────────────────────┐              │
                      │ GCE Compute Engine   │──────────────┘
                      │ (Minecraft Server +  │
                      │  Machine Agent)      │
                      └──────────────────────┘
```

### Components

**Controller** (`cmd/controller/`): Go HTTP server deployed on Cloud Run. Serves the React SPA and REST API. Receives Pub/Sub lifecycle events. Manages Pulumi stacks for each server (create, update, destroy). Handles Google OAuth2 login.

**Machine Agent** (`cmd/machine-agent/`): Go binary running as the startup command on each Minecraft VM. Reports server status (players, uptime, version) to Firestore. Syncs the whitelist between Firestore and Minecraft's `whitelist.json`. Handles scheduled shutdowns (in-game warnings, world save, VM stop). Uses GCE metadata for self-identification.

**Frontend** (`web/`): React 19 SPA built with Vite. Communicates with the controller through a REST API. Polls for server status and provisioning progress.

**Firestore**: Native-mode database storing server config, provisioning status, and runtime status. Database ID format: `{environment}-{region}-metio-db`.

**Pulumi**: Each server has its own Pulumi stack (stored in a GCS state bucket) that defines GCE VM, boot disk, service account, firewall rules, IAM bindings, and backup bucket.

**Pub/Sub**: Forwards compute instance lifecycle events (start/stop/preempt) to the controller.

### Technology Choices

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Backend | Go 1.25, Gorilla Mux | Performance, single binary, good GCP SDK support |
| Frontend | React 19, TypeScript, Vite | Modern tooling, type safety, fast HMR |
| Styling | TailwindCSS 4 | Utility-first, small bundles |
| State | React Query | Server state caching, polling, mutations |
| Auth | Google OAuth2 + cookie sessions | No password management, session-based |
| Orchestration | Pulumi Automation API | Embedded infra-as-code per server |
| Shared infra | OpenTofu | Declarative shared resource management |
| CI/CD | GitHub Actions | GitHub hosting, ghcr.io image registry |

## Code Organization

```
metio/
├── cmd/
│   ├── controller/           # Go HTTP server (Cloud Run)
│   │   ├── main.go           # Entry point, server setup
│   │   ├── Dockerfile        # Multi-stage build (Node → Go → Alpine)
│   │   └── cloudbuild.yaml   # Cloud Build config
│   └── machine-agent/        # VM status reporter
│       ├── main.go           # Entry point, Firestore sync loop
│       ├── main_test.go
│       ├── Dockerfile
│       └── cloudbuild.yaml
├── internal/
│   ├── config/               # Viper-based env var loading
│   │   ├── config.go         # Config struct, Load()
│   │   └── config_test.go
│   ├── db/                   # Firestore data access layer
│   │   ├── db.go             # DB interface
│   │   ├── types.go          # Status, ServerConfig, ProvisioningStatus
│   │   ├── server_config.go  # Server CRUD
│   │   ├── provisioning.go   # Provisioning status operations
│   │   ├── settings.go       # Pulumi settings
│   │   └── validation.go     # Validation data access
│   ├── handlers/             # HTTP handlers + middleware
│   │   ├── auth.go           # OAuth login, callback, session
│   │   ├── base.go           # Router setup, SPA handler
│   │   ├── cors.go           # CORS middleware (dev only)
│   │   ├── events.go         # Pub/Sub lifecycle handler
│   │   ├── middleware.go     # Tracing, auth middleware
│   │   ├── servers/          # Server CRUD + provisioning API
│   │   │   ├── routes.go     # Route registration
│   │   │   ├── crud.go       # List, get, create, update, delete
│   │   │   ├── actions.go    # Start, stop, save world
│   │   │   ├── provisioning.go  # Provisioning status endpoint
│   │   │   ├── common.go     # Shared types and helpers
│   │   │   ├── deps.go       # Dependency injection
│   │   │   └── whitelist.go  # Whitelist management
│   │   └── setup/            # Initial setup wizard
│   │       ├── handler.go
│   │       ├── routes.go
│   │       └── deps.go
│   ├── pulumi/               # Pulumi automation API
│   │   ├── interfaces.go     # WorkspaceManager interface
│   │   ├── workspace.go      # Workspace management
│   │   └── programs/
│   │       ├── server.go     # GCP resources for a server VM
│   │       ├── cloud_config.go  # Cloud-init template
│   │       ├── version.go    # Program version
│   │       └── server_test.go
│   ├── services/             # Business logic
│   │   ├── provisioning.go   # Create/update/destroy orchestration
│   │   ├── setup.go          # Initial project setup
│   │   ├── update_operations.go  # Update type detection
│   │   ├── validation.go     # GCP project validation
│   │   ├── validation_cache.go   # Validation result caching
│   │   ├── mojang.go         # Minecraft username lookup
│   │   └── storage_adapter.go    # Storage client adapter
│   ├── testutil/             # Shared test mocks
│   │   ├── mock_db.go        # Mock DB (testify)
│   │   ├── mock_provisioning.go
│   │   ├── mock_storage.go
│   │   └── mock_workspace.go
│   └── tracing/              # OpenTelemetry
│       ├── tracer.go         # Trace provider setup
│       └── metrics.go        # Metrics registration
├── web/                      # React frontend (Vite)
│   ├── src/
│   │   ├── main.tsx          # Entry point
│   │   ├── App.tsx           # Routing (React Router)
│   │   ├── components/
│   │   │   ├── layout/       # Layout, Header, StatsGrid
│   │   │   ├── server/       # Dashboard, Wizard, Modals
│   │   │   ├── setup/        # Setup wizard
│   │   │   ├── ui/           # Badge, Button, Card, etc.
│   │   │   └── ProtectedRoute.tsx
│   │   ├── hooks/            # Custom hooks (React Query)
│   │   ├── contexts/         # AuthContext
│   │   ├── types/            # TypeScript interfaces
│   │   └── lib/utils.ts     # Tailwind class merging
│   ├── vite.config.ts        # Vite config with API proxy
│   └── vitest.config.ts      # Vitest config (80% coverage)
├── deploy/                   # OpenTofu shared infrastructure
│   ├── main.tf               # Provider, Firestore DB
│   ├── controller.tf         # Cloud Run, IAM, secrets
│   ├── events.tf             # Pub/Sub, log sink
│   ├── firestore.tf          # Rules, indexes
│   ├── pulumi_state.tf       # Pulumi state bucket
│   └── variables.tf
├── docs/                     # Documentation
│   ├── DEPLOYMENT.md         # Production deployment guide
│   └── insomnia/             # API collection (Insomnia)
├── static/dist/              # Built frontend (go:embed)
└── Makefile                  # Build, test, deploy targets
```

## Frontend Development

### Stack

- **React 19** with TypeScript
- **Vite** for dev server and build
- **TailwindCSS 4** with `cn()` utility (`clsx` + `tailwind-merge`)
- **React Query** (TanStack Query) for server state
- **React Router** for client-side routing
- **Lucide React** for icons
- **Vitest** + **@testing-library/react** for testing

### Component Structure

Components live in `web/src/components/` organized by domain:

- `layout/` — Layout shell, Header, StatsGrid
- `server/` — ServerDashboard, ServerSetupWizard, ProvisioningProgress, UpdateModal, DestroyModal, ServerConfigPanel, EmptyState
- `setup/` — SetupWizard
- `ui/` — Shared primitives: Badge, Button, Card, Separator, Skeleton, Switch, Tooltip

Each component has co-located tests (`Component.test.tsx`) and snapshot directories (`__snapshots__/`).

### Custom Hooks

Hooks in `web/src/hooks/` encapsulate API communication via React Query:

| Hook | Purpose |
|------|---------|
| `useAuth` | OAuth session state |
| `useServers` | List all servers |
| `useServerStatus` | Poll single server status |
| `useServerProvisioning` | Poll provisioning progress |
| `useServerMutations` | Start, stop, save world |
| `useServerOptions` | Available server configuration options |
| `useWhitelist` | Server whitelist management |
| `useScheduledShutdown` | Server shutdown schedule |
| `useSetupStatus` | Initial setup wizard state |
| `useInitialize` | Trigger initial setup |

### API Proxy

In development, Vite proxies `/api` and `/auth` to `http://localhost:8080`. See `web/vite.config.ts`.

### Testing

```bash
# Run all frontend tests
cd web && npm run test:run

# Run frontend tests with coverage (80% threshold)
cd web && npm run test:coverage

# Run in watch mode
cd web && npm run test
```

Test patterns:
- Use `vitest` (globals enabled)
- `@testing-library/react` (`renderHook`, `waitFor`, `act`)
- Mock modules with `vi.mock()`
- Wrap React Query hooks in `QueryClientProvider` with `retry: false`
- Mock `global.fetch` for API calls

### Building and Embedding

```bash
cd web && npm run build
```

The built assets go to `static/dist/`, which is embedded into the Go binary via `//go:embed`.

## Backend Development

### Handler Patterns

Routes are registered in `internal/handlers/base.go` using `gorilla/mux`. The pattern:

```go
r.HandleFunc("/api/servers", handlers.ListServers).Methods("GET")
r.HandleFunc("/api/servers", handlers.CreateServer).Methods("POST")
```

All API routes are behind `apiAuthMiddleware`, which validates the session cookie or checks for a `DEV_API_KEY` bearer token.

Sub-packages follow the same pattern:
- `internal/handlers/servers/routes.go` registers server-related endpoints
- `internal/handlers/setup/routes.go` registers setup endpoints

### Firestore Data Access

The `internal/db` package defines a `DB` interface:

```go
type DB interface {
    GetServerConfig(ctx, serverID) (ServerConfig, error)
    SetServerConfig(ctx, serverID, config) error
    DeleteServerConfig(ctx, serverID) error
    ListAllServerIDs(ctx) ([]string, error)
    // ...
}
```

Mock implementations live in `internal/testutil/mock_db.go` for testing. Test files in consuming packages import and re-export the mock.

### Pulumi Infrastructure

Each server gets its own Pulumi stack created by the Automation API:

1. `UpsertStack(serverID, program)` — Creates or fetches the stack
2. `InstallStack(stack)` — Installs dependencies
3. `DeployStack(stack, config)` — Applies the infrastructure
4. `DestroyStack(stack)` — Tears down the resources

The Pulumi program (`internal/pulumi/programs/server.go`) defines:
- GCE VM instance
- Boot disk with Minecraft server image
- Firewall rules (port 25565)
- Service account
- IAM bindings
- Backup storage bucket

### Adding New Features

**New API endpoint:**
1. Add handler function in the appropriate file under `internal/handlers/`
2. Register the route in `routes.go`
3. Add any new Firestore operations in `internal/db/`
4. Add business logic in `internal/services/`
5. Write tests with `httptest` + testify mocks

**New frontend page:**
1. Create the component in `web/src/components/`
2. Add the route in `web/src/App.tsx`
3. Add API hooks in `web/src/hooks/`
4. Add types in `web/src/types/`
5. Write tests with Vitest + testing-library

### Testing

```bash
# Run all Go tests with coverage
make test-backend

# Run a single test package
go test ./internal/services -run TestCreateServer

# Run tests with verbose output
go test ./... -v
```

Go testing conventions:
- Standard library `testing` package
- `github.com/stretchr/testify/assert` for assertions
- `github.com/stretchr/testify/mock` for mocks
- `net/http/httptest` for HTTP tests
- Table-driven tests with anonymous struct slices
- `t.Setenv()` for scoped environment variables

## Contributing Guidelines

### Branch Naming

Branches are created from Linear tickets:

```
metio-<ticket-number>-<kebab-case-title>
```

Example: `metio-98-create-contribution-and-development-guide`

Create branches using the Linear-generated name to maintain traceability.

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <description>

[optional body]
```

Types:
| Type | Usage |
|------|-------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `refactor` | Code change that neither fixes nor adds |
| `test` | Adding or updating tests |
| `chore` | Build process, tooling, dependencies |

Examples:
```
feat: add server start/stop functionality
fix: handle empty whitelist on server creation
docs: add deployment guide
refactor: extract status polling into custom hook
```

### Pull Request Process

1. **Create a pull request** via GitHub:
   ```bash
   gh pr create \
     --base main \
     --title "<commit-type>: <description>" \
     --body "Closes <TICKET-ID>\n\n<summary of changes>"
   ```
2. **Set Linear to "In Review"** after creating the PR
3. **Enable auto-merge** (`gh pr merge --auto --squash`) or wait for manual merge
4. **Enable delete branch** in the PR to ensure the branch is deleted on merge

### Before Submitting

- [ ] Run `make test` (all Go + frontend tests pass)
- [ ] Run `make verify-backend` (`go mod tidy`, `go fmt`, `go vet`)
- [ ] Run `cd web && npm run lint`
- [ ] Run `cd web && npm run format` (Prettier)
- [ ] Run `cd deploy && tofu fmt` if OpenTofu files changed
- [ ] Pre-commit hooks run automatically on `git commit`

### Code Review Expectations

- All code changes require at least one approval
- Tests should accompany new features and bug fixes
- Architecture changes should be discussed before implementation
- Inline comments should be avoided — prefer self-documenting code

### CI/CD Pipeline

On every push and pull request to `main`, GitHub Actions runs (`.github/workflows/ci.yml`):

| Job | Jobs | Trigger |
|-----|------|---------|
| Test | `go test`, `web test`, `lint` | Always |
| Docker | Build + push to ghcr.io | On `main` merge only |

Images are tagged with git SHA. Non-`main` tags are cleaned by ghcr.io retention policy (3 days).

To promote an image for deployment:

```bash
make promote FROM=<sha> TO=main
```
