# Pulumi Cleanup Tool

A manual escape hatch for destroying orphaned per-server Pulumi stacks that the
controller can no longer reconcile — for example, stacks left behind by failed
provisioning (fixed in commit `177abca`).

## Why This Exists

The controller's `WorkspaceManager.DestroyStack`
(`internal/pulumi/workspace.go:70`) handles the happy path. When provisioning
fails partway through, the stack may orphan 17+ GCP resources (firewall, disk,
backup bucket, address, resource policy, service account, IAM bindings, etc.)
without ever reaching a stable state-store record. This tool lets an operator
manually tear down those resources and clean up state.

## Prerequisites

- `pulumi` CLI (v3.x)
- `gcloud` CLI authenticated against `minecraftbyl`
- Write access to the Pulumi state bucket

## Environment

```bash
export PULUMI_BACKEND_URL="gs://development-metio-pulumi-state"
export PULUMI_CONFIG_PASSPHRASE=""
export PULUMI_HOME="/tmp/.pulumi"
```

For other environments, replace `development` with the target environment name
(e.g. `production`). The bucket is configured in `deploy/modules/gcp-cloud-run/pulumi_state.tf` and
exposed as `PULUMI_STATE_BUCKET` in the controller Cloud Run template.

## Usage

```bash
cd tools/pulumi-cleanup

# List all stacks (stack name = server UUID)
pulumi stack ls

# Select the orphaned stack
pulumi stack select 0dcbaca4-2a26-489c-b4a3-d2fad8bb6483

# Tear down all GCP resources
pulumi destroy --yes

# Remove the stack from state
pulumi stack rm 0dcbaca4-2a26-489c-b4a3-d2fad8bb6483 --yes
```

### State Store Cleanup

After destroying the stack, the controller's state record for the server (stored in the
Dapr state store) must also be removed.

**Recommended:** use the controller API (idempotent, exercises the same code
path the user interface calls):

```bash
# Reachable controller (local or Cloud Run):
curl -X DELETE "http://localhost:8080/api/servers/0dcbaca4-2a26-489c-b4a3-d2fad8bb6483"
```

## Safety Note

This tool is **not** embedded into the controller binary.
- `static/embed.go` only embeds `static/dist/`
- `make` / `make all` only compiles `cmd/*/main.go`

It is safe to bundle in the repository — it never affects production builds.
