# Deploying Metio to a New GCP Project

This guide walks through setting up Metio in a Google Cloud Platform project from scratch — from enabling APIs through creating your first Minecraft server.

## Prerequisites

Before you begin, you need a GCP project with [billing enabled](https://cloud.google.com/billing/docs/how-to/modify-project).

### Required Tools

| Tool | Version | Purpose |
|------|---------|---------|
| [gcloud CLI](https://cloud.google.com/sdk/docs/install) | Latest | GCP resource management |
| [OpenTofu](https://opentofu.org/docs/intro/install/) | 1.9+ | Infrastructure deployment |
| [Docker](https://docs.docker.com/engine/install/) | Latest | Building container images |
| Go | 1.25+ | Local builds (optional) |
| Node.js | 20+ | Frontend builds (optional) |

### Required GCP APIs

Enable these APIs in your project:

```bash
gcloud services enable \
  cloudresourcemanager.googleapis.com \
  compute.googleapis.com \
  pubsub.googleapis.com \
  secretmanager.googleapis.com \
  artifactregistry.googleapis.com \
  run.googleapis.com \
  logging.googleapis.com \
  iam.googleapis.com \
  storage.googleapis.com
```

> In `cloudsql` mode, OpenTofu enables `sqladmin.googleapis.com` automatically — you don't
> need to enable it here.

### OAuth Consent Screen

1. Go to **APIs & Services → OAuth consent screen** in the Google Cloud Console
2. Choose **External** user type (or Internal if all users belong to your organization)
3. Fill in the required fields (app name, support email, developer contact)
4. Add the `.../auth/userinfo.email` and `.../auth/userinfo.profile` scopes
5. Add your email as a test user

### OAuth Client Credentials

1. Go to **APIs & Services → Credentials**
2. Click **Create Credentials → OAuth client ID**
3. Choose **Web application**
4. Add the authorized redirect URI: `https://<your-controller-domain>/auth/callback`
5. Note the **Client ID** and **Client Secret** — you will need them later

### GitHub Container Registry

Images are published to `ghcr.io/nbyl/metio`. Ensure you are authenticated:

```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u <your-username> --password-stdin
```

## Controller Deployment

### Step 1: Configure Infrastructure Variables

Copy the sample variables file and edit it with your project details:

```bash
cp metio.auto.tfvars.sample metio.auto.tfvars
```

Edit `metio.auto.tfvars`:

```hcl
project_id    = "your-project-id"
region        = "europe-west3"
zone          = "europe-west3-a"
admin_users   = "your-email@example.com"
postgres_mode = "cloudsql" # or "byo"
```

The `admin_users` variable controls who can log in to the Metio dashboard. Multiple users can be comma-separated.

### PostgreSQL State Backend

Metio stores all state in PostgreSQL, accessed through the Dapr sidecar. The `postgres_mode` variable selects one of two topologies (see [ADR-0003](adr/0003-postgresql-state-backend.md)):

| Mode | Provisioning | Cost | Best for |
|------|-------------|------|----------|
| `cloudsql` | OpenTofu auto-provisions a Cloud SQL Postgres instance and writes the connection string to a Secret Manager secret | ~$7–9+/month, always-on | Single-account simplicity — everything billed on one GCP project |
| `byo` | You supply a Postgres connection string (Neon, CockroachDB, or any Postgres); OpenTofu provisions no database | ~$0 idle (free tier, scales to zero) | Cost-sensitive self-hosters |

In both modes the Dapr `statestore` component reads the connection string from the
`postgres-connection-string` secret (surfaced to daprd via a mounted volume). The state
table (`TEXT PRIMARY KEY` + `JSONB` columns) is **auto-created by Dapr** on startup — no
manual schema setup is required.

#### `cloudsql` mode

The default. `tofu apply` creates the instance, database (`metio`), user, and the
`postgres-connection-string` secret automatically. Nothing else to do.

#### `byo` mode (Neon / CockroachDB / any Postgres)

In byo mode OpenTofu **never sees the connection string** — it only references an
externally managed Secret Manager secret by ID. The secret and its versions must exist
*before* `tofu apply`.

1. Set `postgres_mode = "byo"` in `metio.auto.tfvars` and point
   `postgres_connection_string_secret_id` at the secret that will hold the connection
   string.
2. Create a Postgres database with your provider (e.g. a Neon project or CockroachDB cluster) and note its connection string:
   ```
   postgres://<user>:<password>@<host>:5432/<database>?sslmode=require
   ```
   TLS is required — use `sslmode=require` (or stricter). The host must be reachable from Cloud Run (public endpoint).
3. Create the secret and populate it with your real connection string (a JSON document) **before** applying:
   ```bash
   gcloud secrets create <secret-id>
   echo -n '{"postgres-connection-string":"postgres://user:pass@host:5432/metio?sslmode=require"}' | \
     gcloud secrets versions add <secret-id> --data-file=-
   ```
   Rotate later by adding a new version — OpenTofu mounts `latest`, so no apply is needed.
4. Apply the infrastructure (`tofu apply`). If the referenced secret does not exist, apply
   fails at plan time.

> The secret value must be a JSON document with a single `postgres-connection-string`
> key — that is the contract the mounted secret file is read as.

### Step 2: Deploy Shared Infrastructure

```bash
cd deploy
tofu init
tofu apply
```

This creates a module called `gcp-cloud-run` (from `deploy/modules/gcp-cloud-run/`) which manages all shared infrastructure. If you do not specify `controller_image` or `machine_agent_image`, the module uses the defaults from the latest release.

### Using as a Module in Your Own Repository

You can include the Metio infrastructure module in your own OpenTofu configuration:

```hcl
module "metio" {
  source = "github.com/nbyl/metio//deploy/modules/gcp-cloud-run"

  project_id          = var.project_id
  region              = var.region
  zone                = var.zone
  environment         = var.environment
  admin_users         = var.admin_users
  controller_image    = var.controller_image    # optional — defaults to latest release
  machine_agent_image = var.machine_agent_image # optional — defaults to latest release
}
```

The `//deploy/modules/gcp-cloud-run` path tells OpenTofu to reference the module subdirectory inside the repository.

This creates:
- **PostgreSQL state backend** — in `cloudsql` mode, a Cloud SQL instance + database + user and the connection-string secret; in `byo` mode, only the secret (placeholder) for you to fill
- **Cloud Run service** (controller) with the default release image
- **Pub/Sub topic + subscription** for compute instance lifecycle events
- **Log sink** routing compute audit logs to Pub/Sub
- **Custom IAM role** with permissions for the controller service account
- **Secret Manager secrets** for OAuth credentials and API keys
- **GCS bucket** for Pulumi state storage

### Step 3: Populate Secrets

The OpenTofu deployment creates Secret Manager secrets with dummy values. You must update them with real credentials:

```bash
# Google OAuth Client ID
echo -n "your-client-id" | gcloud secrets versions add development-client_id --data-file=-

# Google OAuth Client Secret
echo -n "your-client-secret" | gcloud secrets versions add development-client_secret --data-file=-

# Base URL of the controller (e.g., https://your-service-xxxxx-ew.a.run.app)
echo -n "https://your-controller-url" | gcloud secrets versions add development-base_url --data-file=-

# Firebase API key (optional, for future features)
echo -n "your-firebase-api-key" | gcloud secrets versions add development-firebase_api_key --data-file=-
```

If you chose `postgres_mode = "byo"`, also fill the `postgres-connection-string` secret
(see [byo mode](#byo-mode-neon--cockroachdb--any-postgres) above). In `cloudsql` mode this
secret is populated automatically by `tofu apply`.

### Step 4: Build and Deploy the Controller Image

```bash
# Build the controller image
make controller-image

# Push to ghcr.io
make push-images
```

Alternatively, build and push a specific tag:

```bash
docker buildx build --platform linux/amd64 \
  -f cmd/controller/Dockerfile \
  -t ghcr.io/nbyl/metio/controller:<tag> .
docker push ghcr.io/nbyl/metio/controller:<tag>
```

Deploy the image to Cloud Run:

```bash
make deploy-controller
```

This runs `tofu apply -target=module.gcp-cloud-run.google_cloud_run_v2_service.controller` with the new image tag.

### Step 5: Configure Domain and SSL (Optional)

By default, the controller is available at a `*.run.app` URL. To make it available under a
custom domain, [Firebase Hosting](https://firebase.google.com/docs/hosting) is the
recommended approach: it works in **all** Cloud Run regions (including `europe-west2`, where
Cloud Run domain mappings are not available) and is not subject to the domain-mapping
preview caveats.

#### Approach A: Firebase Hosting (recommended)

Firebase Hosting proxies every path of your domain to the controller Cloud Run service and
provisions a TLS certificate for you. Your project likely already uses Firebase (the
controller reads `FIREBASE_API_KEY`); if not, [add Firebase to the project](https://firebase.google.com/docs/web/setup) first.

1. Install the Firebase CLI (`firebase-tools`).
2. In a folder **separate** from the Metio source code, create a `firebase.json` that
   rewrites all requests to the controller service:
   ```json
   {
     "hosting": {
       "rewrites": [{
         "source": "**",
         "run": {
           "serviceId": "<environment>-controller",
           "region": "<region>"
         }
       }]
     }
   }
   ```
   Replace `<environment>` and `<region>` with the values from your `metio.auto.tfvars`
   (e.g. `development2-controller` in `europe-west2`).
3. Deploy the hosting configuration:
   ```bash
   firebase deploy --only hosting --project <project_id>
   ```
4. [Connect a custom domain to Firebase Hosting](https://firebase.google.com/docs/hosting/custom-domain)
   and follow the DNS verification steps there.
5. Update the `<environment>-base_url` secret to `https://<your-custom-domain>` and redeploy
   the controller for the change to take effect — the OAuth authorized redirect URI from the
   [OAuth Client Credentials](#oauth-client-credentials) step must match this domain
   (`https://<your-custom-domain>/auth/callback`).

> This is a manual, console-driven flow: Firebase Hosting is not managed by the OpenTofu
> module. For automated provisioning, see
> [Firebase Hosting and Cloud Run](https://firebase.google.com/docs/hosting/cloud-run).

#### Approach B: Cloud Run domain mappings (Preview, not recommended)

Cloud Run's native custom-domain mapping is still in **Preview** — it is not
production-ready and is only supported in a fixed set of regions
(`asia-east1`, `asia-northeast1`, `asia-southeast1`, `europe-north1`, `europe-west1`,
`europe-west4`, `us-central1`, `us-east1`, `us-east4`, `us-west1`). It is **not available
in `europe-west2`**. If your region supports it:

1. Go to **Cloud Run → Your Service → Domain Mappings**
2. Click **Add Mapping** and follow the DNS verification steps
3. Update the `<environment>-base_url` secret to your custom domain
4. Redeploy the controller for the change to take effect

See the [Cloud Run mapping docs](https://docs.cloud.google.com/run/docs/mapping-custom-domains#run)
for the full feature limitations.

### Step 6: First-Time Authentication

1. Navigate to your controller URL (e.g., `https://<your-service>-<hash>-<region>.a.run.app`)
2. You will be redirected to Google to sign in
3. After authentication, the dashboard loads with a prompt to run the **Setup Wizard**

## Server Provisioning

### Setup Wizard

The first time you log in, the dashboard will direct you to the Setup Wizard at `/setup`. The wizard:

1. **Creates a Pulumi state bucket** in GCS (named `{environment}-metio-pulumi-state`)
2. **Verifies GCP API enablement** and project readiness
3. **Saves settings** to the Dapr state store for future use

If the state bucket already exists with proper Metio labels (`managed-by: metio`, `purpose: pulumi-state`), the wizard adopts it.

You can also bypass the wizard by setting the `PULUMI_STATE_BUCKET` environment variable on the controller.

### Creating a Server

1. After setup, click **Create Server** or navigate to `/servers/new`
2. Configure the server:
   - **Server Name**: A unique identifier (e.g., "my-survival-world")
   - **Minecraft Version**: The server version to run (e.g., "1.21.4")
   - **Machine Type**: GCE instance type (e.g., `e2-small`, `e2-standard-2`)
   - **Region/Zone**: Pre-filled from your infrastructure config
3. Click **Create** to start provisioning

The provisioning process:
1. **Upserts a Pulumi stack** in the state bucket
2. **Deploys infrastructure**: GCE VM, boot disk, service account, firewall rules, IAM bindings, backup bucket
3. **Reports progress** via the provisioning status in the state store (polled by the frontend every 2 seconds)

The VM boots with the machine-agent as the startup command, which connects back to the controller to report status.

### Configuration Options

When editing a server (via the dashboard or API), you can change:

| Setting | Description |
|---------|-------------|
| `minecraft_version` | Minecraft server version (e.g., `1.21.4`) |
| `machine_type` | GCE instance type (e.g., `e2-medium`, `e2-standard-4`) |
| `desired_status` | `RUNNING` or `TERMINATED` |
| `rcon_password` | Password for RCON (in-game administration) |
| `whitelist` | List of allowed Minecraft player usernames |

Some changes trigger different update types:
- **In-place** (version, RCON password): Updates the server config, machine-agent picks up the change
- **Resize** (machine type): Stops the VM, changes the machine type, restarts
- **Recreate** (image changes): Destroys and recreates the VM

## Operations Guide

### Starting and Stopping Servers

From the dashboard, use the **Start** / **Stop** buttons on each server card. Alternatively, use the API:

```bash
# Start a server
curl -X POST https://<controller-url>/api/servers/<server-id>/start

# Stop a server
curl -X POST https://<controller-url>/api/servers/<server-id>/stop
```

### Backup Management

Since [ADR-0004](./adr/0004-centralized-backup-catalog-and-restore.md), backups no longer live in a
per-server bucket. A single deployment-wide bucket holds every server's Restic repository under a
per-server prefix:

```text
{project_id}-{environment}-backups            # central bucket (provisioned by OpenTofu)
servers/{server-id}/restic/                   # a server's Restic repository prefix
```

The central bucket and infrastructure are created during `tofu apply`:

- `google_storage_bucket.backups` — the central bucket (`{project_id}-{environment}-backups`). It is
  deliberately **not** `force_destroy`; deleting a server does not delete its backups.
- Secret Manager secret `{environment}-backup-restic-password` — the deployment-wide Restic password
  (randomly generated, not the RCON password). The controller reads it to configure each server's
  `mc-backup` container.
- Bucket IAM: the controller service account gets `roles/storage.objectAdmin` on the central bucket;
   each server VM service account gets object get/create/delete access **scoped to its own prefix**
   (created by the Pulumi server program). Because GCS grants `storage.objects.list` at the bucket
   level and IAM conditions cannot scope it (Restic lists on every `init`/`snapshots`/`prune`), each
   server service account additionally gets a bucket-wide **list-only** custom role
   (`{environment}_backup_object_list`, see `deploy/modules/gcp-cloud-run/backups.tf`) that contains
   just `storage.objects.list` — it can enumerate object names/metadata but not read or modify other
   servers' repositories.

Retention:

- **Active servers**: Restic `--keep-within ${backupRetentionDays}d` (default **90 days**) on each
  server's repository.
- **Deleted servers**: backups are kept for `BACKUP_DELETED_SERVER_RETENTION_DAYS` (default
  **30 days**) after deletion, controlled by the controller. Configure it at deploy time via
  `backup_deleted_server_retention_days` in `deploy/metio.auto.tfvars`.

Backups run on the standard hourly schedule from `mc-backup`. A manual world save can be triggered
from the dashboard with the **Save World** button.

#### Per-server backup settings

Each server inherits the deployment defaults (backups **enabled**, hourly interval, Restic
`--keep-within <backupRetentionDays>d` retention). Individual servers can override the schedule and
retention from the dashboard (the **Backup** section on a server card) or via the API:

```bash
# Read current settings (servers that were never customized report defaults)
curl https://<controller-url>/api/servers/<server-id>/settings/backup

# Override: keep the 3 most recent daily snapshots, backup every 6 hours
curl -X PUT https://<controller-url>/api/servers/<server-id>/settings/backup \
  -H 'Content-Type: application/json' \
  -d '{"enabled": true, "backupIntervalHours": 6, "keep": 3, "keepUnit": "daily"}'

# Disable backups entirely for this server
curl -X PUT https://<controller-url>/api/servers/<server-id>/settings/backup \
  -H 'Content-Type: application/json' \
  -d '{"enabled": false}'
```

The request body maps to the backup schedule and Restic retention:

| Field               | Effect                                | Meaning                                |
|---------------------|---------------------------------------|----------------------------------------|
| `backupIntervalHours` | `BACKUP_INTERVAL`                   | Hours between backups (e.g. `6`, `24`), zero means default `1h` |
| `keep` + `keepUnit` | `--keep-<unit> <keep>`               | Keep the last N snapshots of the given unit (`hourly`, `daily`, `weekly`, `monthly`, `yearly`) |

Zero/empty values fall back to the deployment default for that dimension. Settings are stored per
server in the state store and applied by re-provisioning the VM (a Pulumi deployment appears on the
provisioning page). The `mc-backup` image that renders the schedule/retention configuration is the
Metio image from `cmd/mc-backup/`, which wraps the upstream
[`itzg/mc-backup`](https://github.com/itzg/docker-mc-backup) and adds a post-backup hook — a Go
binary at `cmd/mc-backup/post-backup/` — that writes a
`/manifests/manifest-<timestamp>.json` file (timestamp, snapshot id, server id, repository prefix,
duration, file count, repository size, status) after every successful backup. Each backup produces
its own timestamped manifest, so a slow ingestion process never misses a snapshot because the file
was overwritten. Failed backups write no manifest, so the catalog keeps reporting the last
known-good backup.

The machine-agent mounts the same directory and relays each manifest to the controller's backup
report API (`POST /api/servers/{id}/backups/report`, see ADR-0004). It follows an at-least-once
strategy:

- Manifests are **deleted only after the controller acknowledged** them (any 2xx response), so
  controller outages or agent restarts never lose a record; pending files are simply retried on the
  next tick.
- Duplicate deliveries are harmless: the controller deduplicates reports by server and snapshot ID.
- Unparsable manifests are quarantined as `*.invalid`, manifests the controller permanently rejects
  (HTTP 400) as `*.rejected`; quarantined files stay on disk for inspection and are not retried.

#### Manual Restic access

Operators can inspect a server's repository from a workstation:

```bash
export GCP_PROJECT="<project-id>"
export ENVIRONMENT="<environment>"
export SERVER_ID="<server-id>"

export RESTIC_REPOSITORY="gs:${GCP_PROJECT}-${ENVIRONMENT}-backups:/servers/${SERVER_ID}/restic"
export RESTIC_PASSWORD="$(gcloud secrets versions access latest \
  --secret=${ENVIRONMENT}-backup-restic-password)"

restic snapshots
restic stats --mode raw-data
```

> **Warning**: the deployment-wide password grants access to **every** server repository. Do not run
> `restic forget`/`restic prune` against a repository unless you intend to remove snapshots there.

See [ADR-0004](./adr/0004-centralized-backup-catalog-and-restore.md) for the full design, including
restore/clone flows that build on this infrastructure.

### Updating Infrastructure

#### Controller Update (UI changes)

```bash
make controller-image
make deploy-controller
```

This updates only the Cloud Run service without touching other infrastructure. The frontend is embedded in the Go binary, so UI changes are included.

#### Machine-Agent Update

```bash
make machine-agent-image
make deploy-machine-agent
```

When the machine-agent image changes, the Pulumi program recreates the VM with the new image. Existing servers show an "Update Available" badge in the dashboard.

#### Full System Update

```bash
make deploy
```

Builds all Docker images (controller, machine-agent, mc-backup, daprd) and applies all OpenTofu
infrastructure.

#### mc-backup Image Update

```bash
make mc-backup-image
```

The Metio `mc-backup` image wraps the upstream `itzg/mc-backup` plus a `/manifests` hook (see
[Per-server backup settings](#per-server-backup-settings)). It is pulled by the per-server backup
service at VM boot. Changing the `backup_image` Terraform variable rebuilds the cloud-config pushed
to new/updated servers.

### Monitoring and Logs

- **Controller logs**: View in Cloud Logging with the query `resource.type = "cloud_run_revision" AND resource.labels.service_name = "development-controller"`
- **Server VM logs**: View the machine-agent startup logs with `resource.type = "gce_instance" AND resource.labels.instance_id = "<instance-id>"`
- **Pulumi operations**: Provisioning progress is streamed to the state store and displayed in the frontend
- **Infrastructure state**: View `terraform.tfstate` or use `tofu show`

### Troubleshooting

| Issue | Likely Cause | Solution |
|-------|-------------|----------|
| "Project has no billing account" | Billing not enabled | Enable billing in GCP Console |
| OAuth redirects to localhost | `BASE_URL` secret is wrong | Update `development-base_url` secret |
| "Not authenticated" at login | Email not in `admin_users` | Add your email to `metio.auto.tfvars` and re-run `tofu apply` |
| Server stays in "Provisioning" | Pulumi operation failed | Check Cloud Logging for the controller, or check the server's Pulumi stack directly |
| Cloud Run startup fails | Missing secrets or wrong image | Verify all 4 secrets have current versions, check image exists in ghcr.io |
| State store unreachable | Dapr sidecar failed to start | Check the controller's `daprd` container logs and the `postgres-connection-string` secret / Postgres connectivity |
| Pulumi state locked | Concurrent operation | Wait for the operation to complete or use `pulumi cancel` manually |
| "Instance not found" | VM was manually deleted | Destroy and recreate the server from the dashboard |
| Server backups stop working | Restic password secret missing or stale | Confirm the `{environment}-backup-restic-password` secret has a current version; rotate by adding a new version |

### GitHub Actions CI/CD (Optional)

Metio includes GitHub Actions CI/CD (`.github/workflows/ci.yml`) for automated builds:

1. The workflow runs tests (Go + frontend) on every push and PR to `main`
2. On `main` merges, it builds and pushes Docker images to `ghcr.io/nbyl/metio`
3. Images are tagged with the git SHA
4. Use `make promote FROM=<sha> TO=main` to retag images for deployment

### Architecture Reference

```
┌─────────────┐      ┌──────────────────────────────┐
│   Browser   │──────│   Cloud Run                  │
│  (React UI) │      │  (Controller + API + daprd)  │────┐
└─────────────┘      └──────────────┬───────────────┘    │
                                   │ Pub/Sub              │ Dapr state store
                                   ▼                      │ (PostgreSQL)
                             ┌────────────────────┐       │
                             │ GCE Compute Engine │───────┘
                             │ (Minecraft Server +│
                             │  Machine Agent)    │
                             └────────────────────┘
```

- **Controller** (Go + React): Serves the UI and REST API, manages Pulumi stacks, handles OAuth
- **Machine Agent** (Go): Runs on each VM, reports Minecraft status via the controller API, syncs whitelist, handles shutdowns
- **Dapr state store**: Server config, provisioning status, runtime status (players, uptime) stored via the Dapr sidecar in PostgreSQL
- **Pub/Sub**: Forwards compute instance lifecycle events (start/stop) to the controller
- **Pulumi**: Each server gets its own stack stored in a GCS state bucket
- **OpenTofu**: Manages shared infrastructure (controller, PostgreSQL backend, Pub/Sub, secrets)
