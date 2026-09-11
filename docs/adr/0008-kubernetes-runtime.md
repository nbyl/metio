# ADR-0008: Kubernetes as the Minecraft Runtime

- **Status:** Proposed
- **Date:** 2026-09-11
- **Deciders:** Metio maintainers
- **Relates to:** ADR-0001 (controller/agent API — extended, not superseded)

## Context

Metio runs each Minecraft world on its own preemptible GCE VM. The VM boots
Container-Optimized OS (`internal/pulumi/programs/server.go:386`, `cos-cloud/cos-stable`,
`Preemptible: true`, `ProvisioningModel: SPOT`, `AutomaticRestart: false`) and cloud-init
installs three long-running systemd units, each of which invokes `docker run` directly:
`minecraft.service`, `minecraft-backup.service`, and `metio-machine-agent.service`
(`internal/pulumi/programs/server_cloud_config.yml`).

This arrangement works, but four structural problems have accumulated.

### 1. Any configuration change destroys the VM

`internal/pulumi/programs/server.go:428` declares
`pulumi.ReplaceOnChanges([]string{"metadata.user-data"})` with `DeleteBeforeReplace(true)`.
Because the cloud-config carries the Minecraft version, RCON password, backup interval and
retention, agent image, controller URL and agent token, **changing any of them destroys and
recreates the instance.**

Every recent design has been contorted around this. ADR-0006 accepts a full VM recreation for
a modpack change. ADR-0007 introduces a controller-served `MODS_FILE` endpoint for the sole
purpose of keeping mutable state *out* of `user-data`. ADR-0006's memory decision was chosen
partly because a percentage value keeps `user-data` byte-identical across machine types. This
one line is shaping the product.

### 2. The agent requires root-equivalent host access

`cmd/machine-agent/restore.go:42-57` launches
`docker run --rm --privileged --pid=host --user 0 --entrypoint /usr/bin/nsenter <agentImage>
-t 1 -m -u -i -n /usr/bin/systemctl ...` to stop and start systemd units, which is necessary
only because those units are `Restart=always` and cannot otherwise be held down during a
restore. The agent additionally mounts `/var/run/docker.sock`
(`server_cloud_config.yml:82-112`), which is root-equivalent on the host. Container
supervision is being re-implemented, badly, on top of a system that was not designed for it.

### 3. Secret rotation requires replacing the machine

`AGENT_TOKEN` is an HS256 JWT with a **90-day expiry** (`internal/handlers/agent/auth.go:24-37`)
baked into the cloud-config (`server_cloud_config.yml:107`). Because it lives in `user-data`,
rotating it is a VM replacement. A server left untouched for 90 days has an agent that silently
fails authentication. This is a latent defect, not a hypothetical.

### 4. Supervision is hand-rolled

The cloud-config contains three-attempt `docker pull` retry loops with a
`docker run --entrypoint stat /opt/java/openjdk/bin/java` probe to detect corrupt pulls
(`server_cloud_config.yml:32`), a `ConditionPathExists=!...done` sentinel for run-once restore
(`internal/pulumi/programs/cloud_config.go:104-129`), and a `chmod 0777` directory used as an
IPC channel between the backup container and the agent (`server_cloud_config.yml:114-119`).
These are re-creations of image pull-back-off, init containers, and shared volumes.

### Relationship to ADR-0001

ADR-0001 decided that the machine-agent writes state through a controller HTTP API rather than
directly to the datastore. That decision is **unchanged** here: the agent still reads desired
state from the controller. What this ADR changes is the *execution substrate on the VM*, which
ADR-0001 described in its context but never decided. ADR-0001's decision drivers remain binding
constraints, in particular driver 1: *provider cost must stay near $0 for an idle, single-user
control plane.*

## Decision Drivers

1. **Configuration changes must not destroy the machine.** This is the dominant driver and the
   primary justification for the whole change.
2. **Eliminate privileged host access.** No docker socket, no `--privileged --pid=host`.
3. **Preserve ADR-0001's cost profile.** Near $0 idle for a single-user deployment.
4. **Do not regress the memory budget.** ADR-0006 sets `MAX_MEMORY` as a percentage of machine
   RAM specifically to give Minecraft headroom; a new control plane must not silently consume it.
5. **Survive preemption.** Instances are SPOT with `AutomaticRestart=false` and can be killed at
   any moment.
6. **Leverage an existing ecosystem** rather than continuing to hand-roll supervision,
   scheduling, health checking and secret delivery.

## Considered Options

### A. Status quo

Retain systemd + `docker run`. Zero cost, zero risk, but every problem above persists and
each new feature pays the `ReplaceOnChanges` tax again.

### B. Move mutable configuration out of `user-data`

Generalise the mechanism ADR-0007 already introduces for `MODS_FILE`: serve version, RCON
password, backup settings and agent image from a controller endpoint that the agent polls and
reconciles locally. `user-data` becomes static and `ReplaceOnChanges` stops firing.

This addresses driver 1 — the dominant driver — in roughly **one to two weeks**, and requires no
new control plane, no OS change, and no memory overhead. It does **not** address drivers 2 or 6:
the docker socket, the `nsenter` escalation, and the hand-rolled supervision all remain, and the
90-day token rotation defect is only partially improved.

Option B was initially favoured on the assumption that option C would cost months. On a revised
estimate of roughly four weeks to a working prototype, the cost ratio between them falls to about
three-to-one, while option C addresses every driver rather than one. **Option B is therefore
recorded as the fallback if the spike fails, not as the preferred path.**

### C. Kubernetes as the runtime *(chosen direction)*

Minecraft and its adjacent containers are always scheduled by Kubernetes. Two modes:

- **Standalone** — the controller stays a scale-to-zero Cloud Run service; each server VM runs
  a single-node k3s cluster that manages the Minecraft workload.
- **Cluster** — the controller is installed into an existing Kubernetes cluster alongside
  everything else; Metio does not manage machines at all.

Metio defines its own Kubernetes API objects. The machine-agent evolves into a **metio-operator**
that both translates desired state from the controller into those objects and reconciles them
into Deployments, Services, PVCs and Secrets. See the Decision Outcome for the resulting
component layout.

### D. Podman + Quadlet, or another declarative container manager on the VM

Declarative units without a control plane. Lighter than k3s and removes some supervision code,
but brings a far smaller ecosystem, does not help cluster mode, and still leaves Metio
hand-rolling most orchestration. Not evaluated in depth.

## Decision Outcome

**Chosen direction: C.** Kubernetes becomes the Minecraft runtime, with standalone mode as the
default deployment shape.

**This ADR commits only to a time-boxed spike.** The direction is recorded so that in-flight
work can be sequenced against it, but no implementation beyond the spike is authorised until
the spike reports. The reason is that the single most load-bearing assumption — that k3s runs
acceptably on Container-Optimized OS, on a preemptible VM, without eating the memory budget —
is currently **unverified**, and the codebase has no Kubernetes usage whatsoever today: no
manifests, no `client-go`, no Helm, nothing.

### The spike

**Gating criteria — all three must pass:**

1. k3s installs and survives a reboot on `cos-stable` **without a custom machine image**. The
   known obstacle is that COS mounts `/var` `noexec` while k3s expects to place executables and
   its data root there; the spike must establish whether a supported workaround exists.
2. Cluster state survives hard preemption with the datastore on the attached persistent disk.
3. Backup and restore work **without privileged host access** — no docker socket, no
   `--privileged --pid=host`, no `nsenter`.

**Non-gating measurements the spike must also report:**

4. Idle control-plane RAM and CPU overhead, expressed against the machine's total, so it can be
   weighed against ADR-0006's `MAX_MEMORY` percentage.
5. Whether a Minecraft pod is reachable on 25565 and survives a configuration change **without
   VM replacement** — the primary benefit, which the gating criteria do not otherwise
   demonstrate.

**If criterion 1 fails**, the fallback is **Ubuntu LTS**, accepting the changed patching cadence,
boot time and attack surface. This is recorded now so the spike has a defined branch rather than
stalling on an open question.

**This ADR authorises the spike only.** Its outcome — Container-Optimized OS versus Ubuntu, the
measured control-plane overhead, and the reachability and config-change findings — must land as
an **amendment to this ADR before any build work begins**, in the same way ADR-0006 was amended
by #532.

### Architecture (subject to the spike)

Metio defines its own Kubernetes API objects, so that the desired state of a server is a
first-class, declarative resource rather than a rendered cloud-config string.

These objects are served by a **single binary, `metio-operator`, hosting two controllers**, in the
style of `kube-controller-manager`. The existing `cmd/machine-agent/` becomes
`cmd/metio-operator/`.

- **Sync controller** — reconciles desired state from the controller API into Metio custom
  resources. It is the **sole writer of Metio custom resources in both modes**, applying them with
  Server-Side Apply under a single field manager.
- **Reconcile controller** — reconciles Metio custom resources into Deployments, Services, PVCs
  and Secrets, and writes `.status`. The sync controller reads that status and reports it back to
  the controller API.

**The custom resource is the seam.** Neither controller knows the other exists: the sync
controller's only job is to make the resource match the controller API, and the reconcile
controller's only job is to make the cluster match the resource. The provenance of the object is
irrelevant to reconciliation.

Making the sync controller the sole writer in both modes is deliberate. The alternative — having
the controller write custom resources directly when it runs inside a cluster — produces two code
paths generating the same objects. Metio already carries that defect in the two copies of
`buildProgramConfig` (`internal/handlers/servers/common.go:121` and
`internal/handlers/tasks/handler.go:100`), which must be kept in sync by hand and are a standing
source of bugs. With a single writer, the only differences between modes are *where the binary
runs* and *whether Metio provisions the machine*.

This design yields four properties:

1. **A uniform contract across both modes.** Standalone and cluster mode differ in deployment
   topology, not in data flow.
2. **Level-triggered instead of edge-triggered reconciliation.** Writing a resource is an apply,
   not a command, so re-running is free and recovery after preemption is simply another reconcile.
   This replaces the current edge-triggered `PendingCommand` / `PendingCommandResult` handshake
   (`internal/services/update_operations.go:113`), which polls every two seconds against a
   timeout and must be acknowledged exactly once.
3. **A clean status path.** Status flows outward through the resource rather than through shared
   state between components.
4. **Tolerance of controller unavailability.** The controller is a Cloud Run service scaled to
   zero. If it is unreachable, the custom resource retains the last known desired state and the
   reconcile controller continues to operate.

It also improves debuggability: the desired and observed state of a server become inspectable in
the cluster with standard tooling, rather than living in memory and in log lines.

Two caveats follow from it:

- **In standalone mode the custom resource is a projection, not the source of truth.** The
  controller API remains authoritative, so a manual `kubectl edit` is reverted on the next sync.
  Cluster state is authoritative to *read*, not to *author*. Server-Side Apply with a single
  field manager makes this ownership explicit rather than implicit.
- **The sync controller requires in-cluster credentials** — a ServiceAccount and RBAC permitting
  it to write Metio resources.

**Bootstrapping standalone mode:** k3s automatically applies manifests placed in
`/var/lib/rancher/k3s/server/manifests`. The cloud-config drops the `metio-operator` manifest
there and k3s deploys it on boot, requiring no `kubectl` invocation and no bootstrap job.
Critically, that manifest is **static** — it carries no per-server mutable configuration — so it
does not reintroduce the `user-data` churn this ADR exists to eliminate.

### Cluster mode is a direction, not a commitment

Cluster mode is named here as the long-term destination but **deferred to a later ADR**, because
it is not yet decidable:

- A managed Kubernetes control plane costs roughly **$73/month before any workload runs**, with
  nodes that cannot scale to zero. That is irreconcilable with ADR-0001 driver 1 for a
  single-user deployment.
- Minecraft is raw TCP on 25565 with no HTTP layer. One cloud load balancer per server is
  prohibitively expensive, so cluster mode requires hostname-based TCP routing that inspects the
  Minecraft handshake. That is real, additional machinery with no prototype today.

Cluster mode therefore serves a **different user** — someone already operating a cluster and
hosting many servers — rather than being a second way to serve the same user. The ADR that
specifies it must say so explicitly, or Metio will be defending "$0 idle" and "bring a cluster"
at the same time.

### Migration of existing servers

Existing servers are migrated using the **backup and restore pipeline built in ADR-0004**, which
exists precisely for this scenario. The procedure per server is: take a backup, create a new
server from that backup on the Kubernetes runtime, verify it, then destroy the old one.

This is not new machinery. `CreateServerFromBackup`
(`internal/handlers/servers/backup_catalog.go:338`) already mints a new server ID, provisions
fresh infrastructure, and restores the restic snapshot via `RestoreSnapshotID` and
`RestoreSourcePrefix`, driven from the UI by `web/src/components/backup/CreateFromBackupDialog.tsx`.
Migration is therefore a **runbook over shipped features**, not a workstream: verification,
documentation, and a rollback story, rather than new code.

Two caveats remain. The restore mechanism itself currently renders a one-shot
`metio-restore.service` into the cloud-config (`internal/pulumi/programs/cloud_config.go:104-129`)
and must be re-expressed as an init container — but that work belongs to the runtime change
regardless and is not additional migration cost. And because migration touches user world data,
a verified restore is required before any destructive step.

This removes what would otherwise have been the highest-risk part of the programme.

### Sequencing against ADR-0006

Milestone #9 (ADR-0006, modpack support) is **partially held**, split by whether a ticket depends
on the runtime.

**Proceeding**, because they are runtime-agnostic and survive either spike outcome:

- #533 — sizing the JVM heap from the machine type. Valid under systemd and under Kubernetes, and
  it fixes a defect affecting 30 of 31 machine types today.
- #534 — the Modrinth search service. Pure third-party HTTP client work.
- #535 — the modpack search API endpoints. Controller-side only.

**Held pending the spike**, because they encode the cloud-config runtime:

- #536 — the modpack data model and cloud-config rendering. Its data-model half survives; its
  cloud-config half would be replaced by a field on the custom resource.
- #537 onward, which depend on #536.

Holding the whole milestone was considered and rejected: it would forgo optionality at no saving,
and if the spike fails a user-facing feature would have been stalled for nothing.

## Consequences

### Positive

- `ReplaceOnChanges(["metadata.user-data"])` and the destroy-on-every-config-change behaviour
  are eliminated. Modpack changes, version changes and backup-setting changes become pod updates.
- The `nsenter` privilege escalation and the docker-socket mount are removed.
- `AGENT_TOKEN` becomes a mounted Secret, fixing the 90-day expiry defect.
- Image pull retries, the JVM sanity probe, the run-once restore sentinel and the `0777`
  manifest handoff are replaced by pull-back-off, init containers, and shared volumes.
- Metio's data model becomes declarative and reconciled rather than imperatively provisioned.
- A large ecosystem of operators, integrations and existing knowledge becomes available.

### Negative and risks

- **Minecraft domain logic is unaffected.** Roughly 280 LOC of RCON player counts, version
  parsing, bidirectional whitelist synchronisation against `whitelist.json` and
  `server.properties`, `save-all`, and the three-state shutdown-warning machine must be ported
  from `docker exec` to the Kubernetes exec API, not deleted. It has to live somewhere
  regardless of the runtime.
- **The backup pipeline is ported, not removed.** `cmd/machine-agent/backups.go` (131 LOC),
  `cmd/mc-backup/post-backup/main.go` (233 LOC), restic, and the at-least-once manifest relay all
  survive, re-expressed as a sidecar and a shared volume.
- **Standalone mode still needs the full Pulumi machine layer** — disk, static IP, instance,
  firewall, service account, IAM. `internal/services/provisioning.go` (688 LOC) shrinks but
  remains.
- **The agent's tests are the hidden cost.** `cmd/machine-agent/` is 1,007 LOC of production code
  against **2,339 LOC of tests**. Rewriting the agent means rewriting its test suite, and this is
  the single largest item not visible in a prototype-oriented estimate.
- **k3s overhead partially cancels ADR-0006's memory work**, and may make the smallest machine
  types unviable. The spike must quantify this.
- **A stateful control plane is added to the critical path on hardware chosen to be killed
  arbitrarily.** Preemption is cheap today: the VM reboots and systemd restarts everything.
- **Operator development is a specialist skill** — controller-runtime, finalizers, status
  subresources, idempotent reconciliation. The "lots of knowledge in the wild" argument holds
  strongly for integrations and less so for the core.

### Effort

Standalone mode only, for an engineer familiar with both this codebase and Kubernetes:

| Workstream | Estimate |
|---|---|
| k3s-on-COS spike and cloud-config bootstrap | ~1 week |
| CRD and operator reconcile into Deployment, PVC, Service | ~1 week |
| Agent rewrite: desired state to Metio objects; domain logic via exec API | ~1 week |
| Backup and restore as sidecar plus init container | ~1 week |
| **Working prototype** | **≈ 4 weeks** |
| Test rewrite, documentation, hardening | +2-6 weeks |

The blast radius is narrower than the change first appears. The controller's ~35 HTTP routes, the
authentication model, the Dapr/Postgres state layer and the entire 7,811 LOC frontend are
**untouched** — the API contract does not change. Work is confined to
`internal/pulumi/programs/cloud_config.go`, `server_cloud_config.yml`, `cmd/machine-agent/`, and
the configuration-rendering portions of `internal/services/provisioning.go`.

Variance is dominated by a single unknown: whether k3s runs on Container-Optimized OS. In
particular COS mounts `/var` **`noexec`**, while k3s expects to place executables and its data
root there. This is plausibly a short remount workaround, or the thing that forces Ubuntu. It is
the reason the spike runs first.

### Option B is not foreclosed

Moving mutable configuration out of `user-data` (option B) captures the dominant driver at
roughly a third of the cost and **remains available regardless of the spike outcome**. It is also
complementary: it forces the desired-state boundary to exist as an API, which is precisely the
boundary the operator would later consume. If the spike fails, option B is the fallback and
should be adopted on its own merits.
