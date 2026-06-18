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
  firestore.googleapis.com \
  pubsub.googleapis.com \
  secretmanager.googleapis.com \
  artifactregistry.googleapis.com \
  run.googleapis.com \
  logging.googleapis.com \
  iam.googleapis.com \
  storage.googleapis.com
```

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
cp deploy/metio.auto.tfvars.sample deploy/metio.auto.tfvars
```

Edit `deploy/metio.auto.tfvars`:

```hcl
project_id  = "your-project-id"
region      = "europe-west3"
zone        = "europe-west3-a"
admin_users = "your-email@example.com"
```

The `admin_users` variable controls who can log in to the Metio dashboard. Multiple users can be comma-separated.

### Step 2: Deploy Shared Infrastructure

```bash
cd deploy
tofu init
tofu apply
```

This creates:
- **Firestore database** (native mode) for storing server state
- **Cloud Run service** (controller) with a placeholder image
- **Pub/Sub topic + subscription** for compute instance lifecycle events
- **Log sink** routing compute audit logs to Pub/Sub
- **Custom IAM role** with permissions for the controller service account
- **Secret Manager secrets** for OAuth credentials and API keys
- **GCS bucket** for Pulumi state storage
- **Firestore security rules** and composite indexes

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

This runs `tofu apply -target=google_cloud_run_v2_service.controller` with the new image tag.

### Step 5: Configure Domain and SSL (Optional)

By default, the controller is available at a `*.run.app` URL. For a custom domain:

1. Go to **Cloud Run → Your Service → Domain Mappings**
2. Click **Add Mapping** and follow the DNS verification steps
3. Update the `development-base_url` secret to your custom domain
4. Redeploy the controller for the change to take effect

### Step 6: First-Time Authentication

1. Navigate to your controller URL (e.g., `https://<your-service>-<hash>-<region>.a.run.app`)
2. You will be redirected to Google to sign in
3. After authentication, the dashboard loads with a prompt to run the **Setup Wizard**

## Server Provisioning

### Setup Wizard

The first time you log in, the dashboard will direct you to the Setup Wizard at `/setup`. The wizard:

1. **Creates a Pulumi state bucket** in GCS (named `{environment}-metio-pulumi-state`)
2. **Verifies GCP API enablement** and project readiness
3. **Saves settings** to Firestore for future use

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
3. **Reports progress** via Firestore provisioning status (polled by the frontend every 2 seconds)

The VM boots with the machine-agent as the startup command, which connects back to Firestore to report status.

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

Backups are managed through the Pulumi stack for each server. The backup bucket is created automatically with `ForceDestroy: true` and versioning enabled. Backups run before destructive operations (e.g., machine type changes).

To manually trigger a world save from the dashboard, use the **Save World** button.

### Updating Infrastructure

#### Controller Update (UI changes)

```bash
make build-controller-image
make deploy-controller
```

This updates only the Cloud Run service without touching other infrastructure. The frontend is embedded in the Go binary, so UI changes are included.

#### Machine-Agent Update

```bash
make build-machine-agent-image
make deploy-machine-agent
```

When the machine-agent image changes, the Pulumi program recreates the VM with the new image. Existing servers show an "Update Available" badge in the dashboard.

#### Full System Update

```bash
make deploy
```

Builds both images and applies all OpenTofu infrastructure.

### Monitoring and Logs

- **Controller logs**: View in Cloud Logging with the query `resource.type = "cloud_run_revision" AND resource.labels.service_name = "development-controller"`
- **Server VM logs**: View the machine-agent startup logs with `resource.type = "gce_instance" AND resource.labels.instance_id = "<instance-id>"`
- **Pulumi operations**: Provisioning progress is streamed to Firestore and displayed in the frontend
- **Infrastructure state**: View `deploy/terraform.tfstate` or use `tofu -chdir=deploy show`

### Troubleshooting

| Issue | Likely Cause | Solution |
|-------|-------------|----------|
| "Project has no billing account" | Billing not enabled | Enable billing in GCP Console |
| OAuth redirects to localhost | `BASE_URL` secret is wrong | Update `development-base_url` secret |
| "Not authenticated" at login | Email not in `admin_users` | Add your email to `deploy/metio.auto.tfvars` and re-run `tofu apply` |
| Server stays in "Provisioning" | Pulumi operation failed | Check Cloud Logging for the controller, or check the server's Pulumi stack directly |
| Cloud Run startup fails | Missing secrets or wrong image | Verify all 4 secrets have current versions, check image exists in ghcr.io |
| Firestore permission denied | Rules not deployed | Run `tofu apply` to ensure Firestore rules are active |
| Pulumi state locked | Concurrent operation | Wait for the operation to complete or use `pulumi cancel` manually |
| "Instance not found" | VM was manually deleted | Destroy and recreate the server from the dashboard |
| Backup bucket deletion fails | Objects exist in the bucket | Destroy individual servers first (this cleans up their resources) |

### GitHub Actions CI/CD (Optional)

Metio includes GitHub Actions CI/CD (`.github/workflows/ci.yml`) for automated builds:

1. The workflow runs tests (Go + frontend) on every push and PR to `main`
2. On `main` merges, it builds and pushes Docker images to `ghcr.io/nbyl/metio`
3. Images are tagged with the git SHA
4. Use `make promote FROM=<sha> TO=main` to retag images for deployment

### Architecture Reference

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

- **Controller** (Go + React): Serves the UI and REST API, manages Pulumi stacks, handles OAuth
- **Machine Agent** (Go): Runs on each VM, reports Minecraft status, syncs whitelist, handles shutdowns
- **Firestore**: Server config, provisioning status, runtime status (players, uptime)
- **Pub/Sub**: Forwards compute instance lifecycle events (start/stop) to the controller
- **Pulumi**: Each server gets its own stack stored in a GCS state bucket
- **OpenTofu**: Manages shared infrastructure (controller, Firestore, Pub/Sub, secrets)
