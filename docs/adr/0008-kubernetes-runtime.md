# ADR-0008: Kubernetes as the Minecraft Runtime

- **Status:** Accepted
- **Date:** 2026-09-11
- **Amended:** 2026-09-21 (spike outcome — see "Amendment: Spike outcome")
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

**This ADR committed only to a time-boxed spike.** The direction was recorded so that in-flight
work could be sequenced against it, but no implementation beyond the spike was authorised until
the spike reported. The reason was that the single most load-bearing assumption — that k3s runs
acceptably on Container-Optimized OS, on a preemptible VM, without eating the memory budget — was
**unverified**. The spike is now complete: the assumption held on COS, the preemption behaviour
held, and only the memory budget forced a change (2 GB exclusion). Build work is authorised under
the conditions of the Amendment below. The codebase still has no Kubernetes usage today — no
manifests, no `client-go`, no Helm — which is why the prototype workstreams in the Effort table
start from zero.

### The spike

**Gating criteria — all three must pass:**

1. k3s installs and survives a reboot on `cos-stable` **without a custom machine image** — **PASS**
   (SP2). COS mounts `/var` `noexec` but not globally; a self-managed disk mounted `exec` carries
   both the binary and the data root. Outcome in the Amendment below.
2. Cluster state survives hard preemption with the datastore on the attached persistent disk —
   **PASS** (SP4). Three hard-kill cycles recovered unattended. Outcome in the Amendment below.
3. Backup and restore work **without privileged host access** (no docker socket, no
   `--privileged --pid=host`, no `nsenter`) — **not exercised by the spike**; carried to the
   prototype for early validation (see the Amendment).

**Non-gating measurements the spike must also report:**

4. Idle control-plane RAM and CPU overhead, expressed against the machine's total — **measured**
   (SP5): a roughly constant 435 MB floor. It weighs against ADR-0006's `MAX_MEMORY` percentage;
   the 2 GB machine types are not viable. See the Amendment.
5. Whether a Minecraft pod is reachable on 25565 and survives a configuration change **without
   VM replacement** — **demonstrated** (SP6): version and env changes are pod swaps on an
   untouched VM; the primary benefit this ADR exists for. See the Amendment.

**Operating system — decided at rung 1: Container-Optimized OS.** Criterion 1 passed on COS, so
the ladder was never walked beyond the first rung. The ordering rationale is retained in case a
future runtime needs to revisit the OS choice:

1. **Container-Optimized OS** — the status quo; **chosen**. The expected `/var` `noexec` obstacle
   does not disqualify it: COS enforces `noexec` on its own mounts, not globally, so a
   self-managed disk mounted `exec` carries the k3s binary and data root.
2. **Flatcar Container Linux** — the closest philosophical match for a Kubernetes host, but it
   provisions with **Ignition/Butane rather than cloud-init**, so `cloud_config.go` would be
   rewritten rather than adapted (not applicable: not reached).
3. **Ubuntu LTS** — the cheapest fallback because it consumes cloud-init exactly as COS does, at
   the cost of a larger attack surface, a different patching cadence and slower boot (not
   applicable: not reached).

The ladder is ordered by architectural fit rather than by adoption cost; Ubuntu is the easiest
port but the weakest match for a single-purpose, immutable server image.

**This ADR authorised the spike only.** Its outcome — the chosen operating system, the measured
control-plane overhead, and the reachability and config-change findings — is recorded below as an
amendment, in the same way ADR-0006 was amended by #532. The spike is complete; this amendment is
the go/no-go record and supersedes the provisional wording above.

## Amendment: Spike outcome (2026-09-21)

The time-boxed spike (milestone #10, tickets #542-#547) is complete. Evidence for every claim
below lives on the milestone tickets and their PRs; the harness that produced it was created for
the spike only and has been deleted.

### Gating results

| Criterion | Result | Evidence |
|---|---|---|
| 1. k3s on COS without a custom image | **PASS** (ladder rung 1) | #543/#550 — node Ready 88s cold on stock `cos-cloud/cos-stable`, survives `instances reset`, cloud-init re-runs idempotently |
| 2. Preemption survival | **PASS** | #545/#559 — a real SPOT termination recovered in 134s; a hard reset landing mid-write recovered in 297s (the in-flight 256 Mi write was discarded cleanly); a plain hard reset in 119s. World data, integrity markers and cluster state intact every cycle |
| 3. Backup/restore without privileged access | **Not exercised** | No spike ticket covered it. The restore-as-init-container and backup-sidecar are unverified and must be validated early in the prototype |

### Chosen operating system

**Container-Optimized OS**, decided at ladder rung 1. The central failure hypothesis was wrong in
Metio's favour: COS enforces `noexec` on its *own* mounts, not globally, so a self-managed disk
mounted `exec` can carry the k3s binary and data root. No Flatcar or Ubuntu rung was reached, so
the Ignition consequence does not apply.

The accommodations SP2 (#543) required are stable properties and **none modifies a COS-managed
path**, which is the line between a design choice and a hack:

- the k3s binary, data root and default local-storage path live on the attached
  disk mounted `exec` (this also serves gating 2, which needs the datastore there);
- helper scripts are invoked via `bash` and the k3s installer is piped to `sh`,
  because `noexec` blocks `execve()` but not reading.

These are judged supportable long-term. Two operator caveats were logged: the system
`kubectl` re-extracts to the default `noexec` data dir and the COS-bundled kubectl skews outside
support range — the operator must use the k3s binary or a pinned kubectl.

### Non-gating measurements

**Control-plane overhead (SP5, #546), 10-minute idle window, no workload:**

| scenario | machine | used MB (usable) | k3s overhead | control-plane RSS | idle CPU (sum/peak) |
|---|---|---|---|---|---|
| bare | e2-small | 289 (1973) | — | 42 MB | 0.04% / 0.10% |
| k3s | e2-small | 728 (1973) | 438 MB (22.2%) | 636 MB | 4.38% / 5.25% |
| bare | e2-medium | 362 (3921) | — | 42 MB | 0.05% / 0.15% |
| k3s | e2-medium | 797 (3921) | 435 MB (11.1%) | 618 MB | 4.78% / 5.73% |

The overhead is a **constant ~435 MB floor**, dominated by `k3s-server` (~480 MB RSS), not
proportional to machine size. Consequences for the memory budget:

- **e2-small (2 GB):** 75% heap = 1480 MB vs ~1265 MB actually available after k3s — **not
  viable.**
- **n2-highcpu-2 (2 GB):** same calculus — not viable.
- **e2-medium (4 GB):** 75% heap = 2941 MB vs ~3124 MB available — viable but marginal (183 MB
  headroom, none for JVM native overhead or load spikes).
- **8 GB and up:** comfortably viable.

CPU overhead (< 5% of a 2-4 vCPU machine idle) is negligible against Minecraft's bursty load.

**Config change without VM replacement (SP6, #547), steady e2-medium:**

| change | field | apply → accepting | pod | VM identity | world |
|---|---|---|---|---|---|
| version | `VERSION` 1.21.4 → 1.21.3 | 71s | swapped | id / lastStart / boot_id unchanged, uptime monotonic | intact (4 region files + marker) |
| env | `MAX_MEMORY` 75% → 70% | 69s | swapped | unchanged | intact |

First boot including world generation was 166s, so a restart-only change costs roughly 40% of a
cold boot and **zero VM replacement**. The dominant driver of this ADR is demonstrated
end-to-end: the knobs today's `UpdateTypeRecreate` (`internal/handlers/servers/crud.go:397`)
reacts to are plain pod-spec fields on the Kubernetes path.

### Preemption semantics

SPOT preemption **terminates** the instance (`AutomaticRestart: false`); nothing restarts it, in
the spike or in production today. k3s recovers **unattended** once the instance is started — the
measured recovery times above all include that start. This unchanged-behaviour finding means the
production plan must include an explicit instance-restart path (it must today too; it is not a
regression this ADR introduces).

### Recommendation

**Proceed with changes.** Both exercised gating criteria pass and the primary driver is
demonstrated. Status is therefore **Accepted**, with three conditions:

1. **Exclude the 2 GB machine types** (e2-small, n2-highcpu-2) from the Kubernetes runtime, or
   step `MAX_MEMORY` down for them. This resolves the open question in #533 with a measured
   requirement.
2. **Validate backup/restore without privileged access** (gating 3) early in the prototype. It
   was not exercised by the spike.
3. **Provide an explicit instance-restart path** for preemption termination (see the preemption
   semantics above).

Option B (mutable configuration out of `user-data`) is not foreclosed and remains complementary —
it forces the desired-state boundary as an API, which the operator consumes anyway — but it is no
longer the fallback for this decision.

### Downstream re-assessment

- **#536** — its data-model half proceeds; its cloud-config rendering half becomes a field on the
  Metio custom resource (build work is now authorised by this amendment).
- **#538** — destructive-change confirmation on the current runtime stays (until migration); on
  the Kubernetes runtime the "this destroys your VM" warning becomes unnecessary, because
  configuration changes stop replacing machines.
- **#533** — proceeds regardless; the spike gives it a concrete requirement (2 GB exclusion or
  stepped-down `MAX_MEMORY`).
- **ADR-0007** — its `MODS_FILE` mechanism exists solely to avoid `ReplaceOnChanges`; that
  constraint disappears on the Kubernetes runtime, so the ADR must be reconsidered rather than
  accepted as written.

### Architecture (as amended)

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

ADR-0007 (per-mod and per-plugin selection) is likewise held at `Proposed`. Its `MODS_FILE`
mechanism exists solely to keep mutable state out of `user-data` and therefore avoid
`ReplaceOnChanges`; if the spike passes, that constraint disappears and the ADR should be
reconsidered rather than accepted as written.

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
- **k3s overhead partially cancels ADR-0006's memory work.** The spike quantified it: a constant
  ~435 MB floor, which makes the 2 GB machine types (e2-small, n2-highcpu-2) unviable at
  `MAX_MEMORY=75%` and leaves e2-medium (4 GB) marginal. The small types must be excluded from
  the Kubernetes runtime, or `MAX_MEMORY` stepped down for them (see the Amendment).
- **A stateful control plane is added to the critical path on hardware chosen to be killed
  arbitrarily.** Preemption terminates the instance; recovery depends on an explicit restart
  (the spike measured k3s recovering unattended once restarted — see the Amendment).
- **Operator development is a specialist skill** — controller-runtime, finalizers, status
  subresources, idempotent reconciliation. The "lots of knowledge in the wild" argument holds
  strongly for integrations and less so for the core.

### Effort

Standalone mode only, for an engineer familiar with both this codebase and Kubernetes:

| Workstream | Estimate |
|---|---|
| k3s-on-COS spike and cloud-config bootstrap | ~1 week — **done** (SP1-SP6; estimate held) |
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

Variance was dominated by a single unknown — whether k3s runs on Container-Optimized OS given
that COS mounts `/var` **`noexec`**. The spike resolved it: COS enforces `noexec` on its own
mounts, not globally, so the k3s binary and data root moved to a self-managed disk mounted `exec`.
No remount hack was needed and no ladder rung below COS was required (see the Amendment).

### Option B is not foreclosed

Moving mutable configuration out of `user-data` (option B) captures the dominant driver at
roughly a third of the cost and **remains available regardless of the spike outcome**. It is also
complementary: it forces the desired-state boundary to exist as an API, which is precisely the
boundary the operator would later consume. It is no longer the fallback for this decision (see
the Amendment) but should be adopted on its own merits if the prototype stalls.
