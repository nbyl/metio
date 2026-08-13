# ADR-0004: Centralized Backup Catalog and Restore

- **Status:** Accepted
- **Date:** 2026-08-13
- **Deciders:** Metio maintainers

## Context

Each server currently provisions its **own GCS backup bucket** and runs `itzg/mc-backup`
with a Restic repository scoped to that bucket. Backups are only reachable by operators
via `gsutil`/`restic` against the server's bucket; there is **no catalog** of snapshots, no
retention beyond Restic's `--keep-within 3m`, and when a server is deleted its backup
bucket and snapshots are destroyed along with the VM (`internal/pulumi/programs/server.go`,
`DestroyModal`).

This leaves real operational gaps:

- **No visibility** — users cannot see when a backup ran, what is in it, or how big the
  repository is.
- **No restore / clone UX** — there is no API or UI to restore a snapshot onto an existing
  server or seed a new server from a backup.
- **Retention is weak and per-instance** — `--keep-within 3m` keeps at most a few months
  and is configured identically for every server.
- **Deleted servers lose their backups** — a deleted server's world is gone unless an
  operator manually downloaded the bucket first.
- **Manual-only access** — no documented, repeatable way to reach a server's repository
  from a workstation.

We want backups to be a first-class, cataloged feature of the platform: automatically
recorded, visible in the web UI, restorable onto existing or new servers, and retained for
a controlled window after server deletion.

## Decision Drivers

In priority order:

1. **End-user usefulness** — backups must be discoverable, inspectable, and restorable
   from the UI; operators must be able to restore or clone worlds.
2. **Data durability** — backups must survive server deletion for a configurable window.
3. **Maintainability** — one central store and catalog instead of N bespoke buckets with
   per-instance scripts.
4. **Cost** — a single bucket + object-lifecycle policies keep storage costs near zero for
   small deployments.
5. **Low engine churn** — keep using `itzg/mc-backup` (saving, pause, scheduling, Restic
   pruning) rather than replacing it.

## Considered Options

### (a) Keep per-server buckets; add a controller-side catalog on top

Keep the existing one-bucket-per-server layout and have the agent report snapshots to a
catalog, restoring/cleaning per server.

- Pro: least infra change.
- Con: **does not survive deletion** — the bucket dies with the server; deleted-server
  retention is impossible; more buckets to manage; no single place to enforce global
  retention.

### (b) Single central bucket with per-server Restic prefixes *(chosen)*

One deployment-wide bucket `{project}-{environment}-backups`. Each server's repository
lives under `servers/{server-id}/restic/`. A controller-owned catalog in the database
records every snapshot. `itzg/mc-backup` still runs inside the server and reports through
its **post-backup hook** to the machine-agent, which relays manifests to the controller.
Retention is driven by the controller (per-server active retention + deployment-level
deleted-server retention).

- Pro: backups outlive the server; single cost center; global retention policy; clean
  restore/clone story; per-prefix Restic pruning keeps repos isolated.
- Con: central bucket becomes a chokepoint (mitigated by prefix-isolated repos and
  idempotent cleanup); needs a report path from server → controller.

### (c) Use GCS object lifecycle rules for retention

Rely purely on bucket lifecycle rules (`age > N days`).

- Pro: no controller code.
- Con: **not tied to server deletion state** — cannot express "keep X days after the
  server is deleted"; cannot skip active servers; lifecycle rules cannot reach into Restic
  internals (restic blobs are shared between snapshots, so deleting by age is unsafe).
- Retained only as a coarse safety net / backup-of-the-backup, not the retention mechanism.

## Decision Outcome

**Chosen option: (b) — a single central backup bucket with per-server Restic prefixes and a
controller-owned backup catalog.**

### Central bucket and prefixes

Replace per-server buckets with one deployment-wide bucket:

```text
{project}-{environment}-backups
```

Each server gets its own Restic repository under a server-specific prefix:

```text
servers/{server-id}/restic/
```

Per-prefix repositories keep `mc-backup`'s pruning (`forget`/`prune`) isolated to that
server while all storage shares one bucket. The bucket is provisioned by the controller
infrastructure and is **not** destroyed when an individual server is deleted.

### Backup configuration and retention

- Add `backupRetentionDays` to the server configuration (default **90 days**). The
  generated cloud config passes it to Restic:

  ```text
  PRUNE_RESTIC_RETENTION="--keep-within ${backupRetentionDays}d"
  ```

  This replaces the current hard-coded `--keep-within 3m`.
- Deleted-server retention is **deployment-level** (configured on the controller, e.g.
  `BACKUP_DELETED_SERVER_RETENTION=30d`, **default 30 days**).
- One **deployment-wide Restic password** in Secret Manager (e.g.
  `{environment}-backup-restic-password`); it grants access to all server repositories.

### Backup engine and reporting

Continue to use **`itzg/mc-backup`** as the engine. Its supported `POST_BACKUP_SCRIPT_FILE`
hook runs after each backup. The hook:

1. Verifies the backup exit code.
2. Runs Restic to identify the new snapshot and collect **file count, repository size,
   duration**.
3. Writes a structured completion manifest into a shared machine-agent directory.
4. The machine-agent submits the manifest to `POST /api/servers/{id}/backups/report`
   (authenticated via the existing agent token, **idempotent**).
5. The controller persists the catalog record; the agent removes the manifest only after
   the controller acknowledges it, so a temporarily unavailable controller does not lose
   metadata.

### Catalog record

The controller stores each snapshot in its database (via the existing Dapr/`DaprDB`
adapter):

```text
Backup {
  id
  serverID
  serverName
  snapshotID
  repositoryPrefix
  createdAt
  duration
  fileCount
  repositorySize
  minecraftVersion
  serverDeletedAt
  retentionUntil
  status
}
```

- `(serverID, snapshotID)` is unique.
- When a server is deleted, its catalog records are kept and marked deleted; the source
  server's relevant configuration stays attached so a deleted server's backup can still
  seed a new server.

### API

```text
GET  /api/servers/{server-id}/backups
POST /api/servers/{server-id}/backups/{backup-id}/restore
GET  /api/backups
POST /api/backups/{backup-id}/servers
POST /api/servers/{server-id}/backups/report          (machine-agent, idempotent)
```

Restore and create-from-backup are asynchronous operations surfaced through the existing
provisioning-status model.

### Restore onto an existing server

1. User selects a backup on the server's backup page.
2. UI shows snapshot statistics and a **Minecraft-version mismatch warning** (warn only,
   never block).
3. User confirms the destructive operation.
4. Controller stops the server if necessary and triggers a final world save via the
   existing `BackupCoordinator`.
5. Agent moves the current world data to a recovery directory.
6. Agent restores the selected Restic snapshot.
7. Agent reports completion; controller starts the server and waits for health.
8. UI displays the result.

The selected backup is **never deleted**; on failure the moved current data remains
recoverable.

### Create a new server from a backup

The global backup page offers **Create Server**. The existing server setup wizard opens
pre-filled from the backup (Minecraft version, disk size, region/zone, machine type, source
backup metadata). The user provides a new server name and may override infrastructure
values. Provisioning creates the infrastructure, configures the central repo prefix + the
deployment-wide password, restores the snapshot **before** Minecraft starts, starts the
server/agent, and reports progress through the existing provisioning UI. A Minecraft-version
mismatch warns but does not block.

### Deletion and retention

On server deletion:

- VM and per-server infrastructure are destroyed.
- The central repository and catalog records are **kept**; records are marked deleted and
  `retentionUntil = deletedAt + {deployment-level retention}`.
- Source server configuration remains attached to the backup records.

A controller-driven cleanup worker (scheduled) finds deleted-server backups whose
`retentionUntil` has passed, deletes `servers/{server-id}/restic/` from the central bucket,
removes the catalog records and manifests, and retries failed deletions idempotently.

GCS lifecycle rules are **not** the retention mechanism (they cannot express deletion-state
retention and cannot safely prune shared restic blobs); they may be configured only as an
optional coarse safety net.

### UI

- Per-server backup page (or expandable backup section) with snapshot statistics and
  retention state.
- Global `/backups` page filterable by server, **including deleted servers**.
- Deleted sources are clearly marked, e.g.:

  ```text
  Deleted server: Survival World
  Retention until: 2026-09-12
  ```

- Restore confirmation flow; create-server-from-backup flow; loading/failure/empty states.

### Manual Restic access

Document how operators access the central bucket manually, e.g.:

```bash
export RESTIC_REPOSITORY="gs:${GCP_PROJECT}-${ENVIRONMENT}-backups:/servers/${SERVER_ID}/restic"
export RESTIC_PASSWORD="$(gcloud secrets versions access latest \
  --secret=${ENVIRONMENT}-backup-restic-password)"

restic snapshots
restic stats --mode raw-data
```

Docs also cover GCP credential setup, choosing a server prefix, listing snapshots, viewing
statistics, restoring to a local directory, and a warning to avoid `forget`/`prune` unless
intended (the deployment-wide password grants access to **all** server repositories).

## Consequences

### Positive

- **Backups survive server deletion** with a controlled retention window.
- **Visibility and restore UX** — snapshots, statistics, restore, and clone from the UI.
- **Single cost center** — one bucket; prefix-isolated repos keep pruning simple.
- **Global retention policy** — per-server active retention + deployment-level deleted
  retention, enforced by the controller.
- **Low engine churn** — `itzg/mc-backup` and Restic continue to do the heavy lifting via
  its supported post-backup hook.

### Negative / tradeoffs

- **Central bucket is a chokepoint** — mitigated by per-prefix repos and idempotent,
  paginated cleanup; no cross-server sharing inside Restic.
- **New report path** — the server must relay manifests to the controller; ack-based
  manifest removal prevents metadata loss during controller downtime.
- **Deployment-wide Restic password** — one credential unlocks all repositories; kept as a
  deliberate simplicity tradeoff (per-server passwords rejected).
- **No automatic migration** — existing per-server buckets are not migrated; migration, if
  needed, is manual (see below).

## Migration

**No automatic migration is planned.** Existing per-server repositories may be migrated
manually by operators who want to keep old snapshots: copy the repository contents into
`servers/{server-id}/restic/` of the central bucket, then (optionally) delete the old
bucket. New backups land in the central bucket from rollout onward.

## Implementation impact

- `internal/db/` — `Backup` type + catalog CRUD/list + retention/deleted metadata.
- `internal/handlers/servers/` — backup routes, request/response types, handlers
  (`routes.go`, `crud.go` `DeleteServer` stops destroying backups and marks records).
- `internal/services/` — backup catalog service, restore orchestration, deleted-server
  cleanup worker.
- `internal/agentclient/` — backup report + restore command support.
- `cmd/machine-agent/main.go` — detect post-backup manifests, submit reports, run restores.
- `internal/pulumi/programs/server.go` — remove per-server bucket; pass central bucket,
  repo prefix, retention, password.
- `internal/pulumi/programs/cloud_config.go` + `server_cloud_config.yml` — central Restic
  repo, `POST_BACKUP_SCRIPT_FILE`, manifest directory + hook script,
  `PRUNE_RESTIC_RETENTION` from `backupRetentionDays`.
- `web/src/` — backup types, queries, mutations, per-server + global backup pages, restore
  and clone flows.
- `deploy/` — central backup bucket + IAM, deployment-wide Restic password secret,
  `BACKUP_DELETED_SERVER_RETENTION`.
- `docs/DEPLOYMENT.md` — configuration, UI workflows, retention, manual Restic access.

## Related

- ADR-0001 — Controller-mediated agent state access (gates the report path)
- ADR-0002 / ADR-0003 — state store (catalog records live in the Dapr/PostgreSQL store)
- `internal/pulumi/programs/server.go` — current per-server backup bucket (removed)
- `internal/pulumi/programs/server_cloud_config.yml` — current `mc-backup`/Restic config
  (reworked)
- `docs/DEPLOYMENT.md` — "Backup Management" section (reworked)
