# Backup Operations

This guide covers day-to-day operations for Metio backups — inspecting repositories,
restoring snapshots, creating servers from backups, and migrating legacy per-server buckets.

For infrastructure setup and retention configuration, see [DEPLOYMENT.md](DEPLOYMENT.md)
and [ADR-0004](adr/0004-centralized-backup-catalog-and-restore.md).

## Repository Layout

All backups live in a single GCS bucket provisioned during deployment:

```text
{project_id}-{environment}-backups     # central bucket
├── servers/
│   ├── {server-id-1}/restic/          # server 1's Restic repository
│   ├── {server-id-2}/restic/          # server 2's Restic repository
│   └── ...
```

Each server's Restic repository is isolated under its own prefix. The deployment-wide
Restic password (stored in `{environment}-backup-restic-password` in Secret Manager)
grants access to all repositories.

## Manual Restic Access

Operators can inspect or restore snapshots from a workstation with the
[gcloud CLI](https://cloud.google.com/sdk/docs/install) and
[Restic](https://restic.net/).

### Prerequisites

- `gcloud` authenticated with an account that has `roles/storage.objectViewer` on the
  backup bucket.
- Restic installed (`brew install restic` on macOS, `apt install restic` on Debian/Ubuntu).

### Set Up Environment

```bash
export GCP_PROJECT="<project-id>"
export ENVIRONMENT="<environment>"
export SERVER_ID="<server-id>"

export RESTIC_REPOSITORY="gs:${GCP_PROJECT}-${ENVIRONMENT}-backups:/servers/${SERVER_ID}/restic"
export RESTIC_PASSWORD="$(gcloud secrets versions access latest \
  --secret=${ENVIRONMENT}-backup-restic-password)"
```

### Common Commands

```bash
# List all snapshots for this server
restic snapshots

# Show repository size (deduplicated)
restic stats --mode raw-data

# List files in the latest snapshot
restic ls latest

# Restore the latest snapshot to a local directory
mkdir -p /tmp/restore
restic restore latest --target /tmp/restore

# Restore a specific snapshot (use the snapshot ID from `restic snapshots`)
restic restore abc1234 --target /tmp/restore
```

### Cross-Server Restore

To restore a snapshot from one server into another server's repository (for cloning):

```bash
# Set up the source server's repository
export RESTIC_REPOSITORY="gs:${GCP_PROJECT}-${ENVIRONMENT}-backups:/servers/${SOURCE_SERVER_ID}/restic"

# List snapshots
restic snapshots

# Copy a snapshot to the target server's repository
# (requires both repositories accessible with the same password)
restic --repo "gs:${GCP_PROJECT}-${ENVIRONMENT}-backups:/servers/${SOURCE_SERVER_ID}/restic" \
  copy --repo2 "gs:${GCP_PROJECT}-${ENVIRONMENT}-backups:/servers/${TARGET_SERVER_ID}/restic" \
  latest
```

### Warnings

- **Deployment-wide password**: The Restic password grants access to **all** server
  repositories. Do not run `restic forget` or `restic prune` against a repository unless
  you intend to permanently remove snapshots there.
- **Restore overwrites data**: Running `restic restore` with `--target /data` overwrites
  existing files in the target directory. Always restore to a temporary directory first.
- **Restic cache**: Restic caches repository metadata in `~/.cache/restic`. If switching
  between repositories, use `--cache-dir` or `RESTIC_CACHE_DIR` to avoid conflicts.

## Retention Policies

### Active Servers

Each server's Restic repository uses the configured retention period (default 90 days).
The `mc-backup` container runs `restic forget` with `--keep-within ${backupRetentionDays}d`
after each backup.

Per-server retention can be customized via the dashboard or API:

```bash
# Keep the 3 most recent daily snapshots
curl -X PUT https://<controller-url>/api/servers/<server-id>/settings/backup \
  -H 'Content-Type: application/json' \
  -d '{"enabled": true, "keep": 3, "keepUnit": "daily"}'
```

### Deleted Servers

When a server is deleted, its catalog records are preserved with a retention window
(default 30 days, configurable via `backup_deleted_server_retention_days`). After
retention expires, the controller's cleanup job removes the Restic repository from the
bucket.

To check when a deleted server's backups will be removed, use the global backups API:

```bash
curl https://<controller-url>/api/backups | jq '.[] | select(.serverDeletedAt != null) | {
  serverName: .serverName,
  deletedAt: .serverDeletedAt,
  retentionUntil: .retentionUntil
}'
```

## UI Workflows

### Per-Server Backups

Open the server's update modal and select the **Backups** tab. This shows a list of
snapshots for that server with creation time, duration, file count, size, and Minecraft
version. Click **Restore** on any snapshot to restore it.

### Global Backup Catalog

Navigate to `/backups` to see all backups across all servers. Use the **All** / **Active** /
**Deleted** filter tabs to narrow results. Deleted servers are marked with a red badge.

### Restore a Backup

1. Click **Restore** on a snapshot (from the server's backup tab or the global catalog).
2. Review the snapshot details and version information.
3. If the snapshot's Minecraft version differs from the server's current version, a
   warning is displayed (restore proceeds but may cause compatibility issues).
4. Confirm the restore. The server is stopped, the world is backed up, the snapshot is
   restored, and the server restarts.

### Create a Server from a Backup

From the global backup catalog, click **Create Server** next to a snapshot. The server
creation form opens pre-filled with the backup's source configuration (Minecraft version,
disk size, region, machine type). Provide a new server name and adjust any settings as
needed.

## Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| `restic: permission denied` | Service account lacks bucket access | Verify IAM bindings; the server SA needs `storage.objects.{get,create,delete}` on its prefix |
| `repository does not exist` | Server was never backed up or repository was pruned | Create a manual backup via the dashboard's **Save World** button |
| Snapshot shows `0 files` | Backup is still in progress or post-backup hook failed | Check `mc-backup` container logs on the server VM |
| `restic: key material not found` | Wrong password or wrong repository | Ensure `RESTIC_PASSWORD` matches the deployment-wide secret |

## Legacy Per-Server Buckets

Servers created before the centralized backup architecture (ADR-0004) may still have
backups in per-server GCS buckets (`{project}-{env}-backups-{server-name}`). These
buckets are **not** managed by the cleanup job and will persist until manually deleted.

### Migrating a Legacy Repository

To migrate an old per-server bucket into the central bucket:

```bash
# 1. List objects in the legacy bucket
gsutil ls -r "gs://${LEGACY_BUCKET}/"

# 2. Copy the Restic repository into the central bucket
gsutil -m cp -r "gs://${LEGACY_BUCKET}/" \
  "gs:${GCP_PROJECT}-${ENVIRONMENT}-backups:/servers/${SERVER_ID}/restic/"

# 3. Verify the migrated repository
export RESTIC_REPOSITORY="gs:${GCP_PROJECT}-${ENVIRONMENT}-backups:/servers/${SERVER_ID}/restic"
export RESTIC_PASSWORD="$(gcloud secrets versions access latest \
  --secret=${ENVIRONMENT}-backup-restic-password)"
restic snapshots

# 4. (Optional) Delete the legacy bucket after confirming the migration
gsutil -m rm -r "gs://${LEGACY_BUCKET}/"
```

> **Note**: Migration is manual and one-time. New backups will use the central bucket
> automatically after the server is re-provisioned or updated.
