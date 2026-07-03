# ADR-0001: Machine-agent writes state through a Controller API instead of directly to the datastore

- **Status:** Proposed
- **Date:** 2026-07-03
- **Deciders:** Metio maintainers

## Context

Metio is open-source and **self-hosted per user**: every user runs their own control
plane in their own cloud project. This makes per-user provider cost and setup
simplicity the dominant concerns, ahead of things like multi-tenant scale.

The control plane today consists of:

- A **Controller** running on Cloud Run with `min_instance_count = 0` (scales to zero,
  ~$0/month idle) — see `deploy/modules/gcp-cloud-run/controller.tf:163`.
- **Firestore** as the state store, plus Pub/Sub and Cloud Tasks for lifecycle events.
- A **machine-agent** running on every per-world GCE VM.

Two properties of the workload shape the options below:

- Minecraft uses a **custom TCP protocol** exposed directly on the VM's external
  `IP:25565` via a firewall rule (`internal/pulumi/programs/server.go:150-170`) — **no
  load balancer or HTTPS** is involved anywhere in the data path.
- **World-per-VM is not a hard requirement.** Today each world runs on its own Spot VM,
  but co-locating multiple worlds on shared hosts is acceptable.

The machine-agent is currently coupled to GCP in three ways:

1. It writes state **directly to Firestore** via Application Default Credentials
   (`cmd/machine-agent/main.go:92`).
2. It reads instance identity from the **GCE metadata server**.
3. It **self-stops** the VM through the Compute API (`cmd/machine-agent/main.go:296`).

To do this the per-world VM service account is granted a broad `cloud-platform` scope
plus `roles/datastore.user` (`internal/pulumi/programs/server.go:210-215`). In practice
the agent uses only **~6 of the 20 methods** on the `db.DB` interface
(`internal/db/db.go`) — a narrow, status-and-whitelist-shaped surface.

Separately, an active initiative (#252–259) introduces a **Dapr state-store
abstraction** so the datastore backend becomes pluggable. Issue #259 currently proposes
running `daprd` as a systemd unit **on every VM** so the agent can talk to Dapr locally.

## Decision Drivers

In priority order:

1. **Provider cost** — must stay near $0 for an idle, single-user control plane.
2. **Cloud lock-in / portability** — reduce hard dependencies on GCP-specific services.
3. **Maintainability / architecture** — fewer integration points, clearer boundaries.
4. **End-user simplicity** — easy for a non-expert to self-host.

> **Security is explicitly _not_ a decision driver** for this ADR. It is not ignored,
> but it does not steer the choice between the options below.

## Considered Options

### (a) Leave as-is — agent writes directly to the datastore

The agent keeps direct Firestore access via ADC.

- Cost: ~$0/month idle (unchanged).
- Portability: **poor** — every agent is bound to Firestore + GCP IAM; the Dapr
  migration (#252–259) must be rolled out to the agent on every VM (motivating the
  daprd-on-VM approach in #259).
- Maintainability: two independent datastore integration points (controller + agent).
- Simplicity: fine today, but the pluggable-backend future pushes complexity onto VMs.

### (b) Controller API — agent becomes a thin HTTP client *(chosen)*

The agent stops touching the datastore. It calls a small set of controller HTTP
endpoints for the ~6 operations it actually needs (status updates, whitelist reads).
The controller remains the **single** component that reads/writes state.

- Cost: ~$0/month idle. The agent polls (~30s); a poll may cold-start Cloud Run, but the
  marginal cost is negligible and scale-to-zero is preserved.
- Portability: **best** — the agent has no datastore/IAM coupling; swapping the backend
  (e.g. via Dapr) touches the controller only.
- Maintainability: one datastore integration point; the VM SA can drop
  `roles/datastore.user`.
- Simplicity: the agent ships as a self-contained HTTP client with no cloud SDK state
  wiring.

### (c) Kubernetes (GKE Autopilot + free tier) — *deferred, not rejected*

Move the control plane and worlds onto GKE, exposing Minecraft via **NodePort** (node
`IP:port`, the direct analog of today's model — **no Load Balancer**).

- Cost floor **≈ $10–12/month**. The GKE free-tier credit ($74.40/month per billing
  account) waives the **cluster management fee** for one cluster, and NodePort ingress is
  **$0**. However, an **always-on controller pod** (Autopilot minimum 0.25 vCPU / 0.5 GiB)
  costs ~$10/month in us-central1 and modestly more in `europe-west3`, because vanilla
  Deployments **do not scale to zero**.
- **Upside:** since world-per-VM is not required, worlds can be **bin-packed** onto shared
  nodes, which can beat 1-VM-per-world workload cost for users running several concurrent
  worlds.
- **Why deferred:** for the target case (single-user, frequently-idle control plane) the
  loss of scale-to-zero means ~$10–12/month vs ~$0, plus added operational complexity for
  non-expert self-hosters. It remains a **legitimate future option** — revisit via a
  follow-up ADR/spike if multi-world usage or portability goals grow.

## Decision Outcome

**Chosen option: (b) Controller API.**

The machine-agent will perform all state changes through a narrow, authenticated
controller HTTP API rather than writing to Firestore directly. This best satisfies the
top drivers: it keeps the control plane at ~$0/month idle (Cloud Run scale-to-zero is
preserved), removes the agent's GCP-datastore coupling for portability, and collapses
state access to a single integration point in the controller.

Crucially, (b) **synergizes with the Dapr initiative**: once only the controller talks to
the datastore, the pluggable-backend work (`DaprDB` adapter #256, `DB_BACKEND` selection
#257) applies to the controller alone.

Option (c) is **deferred as future work**, not rejected: (b) is compatible with a later
substrate move and with multiple worlds per host, so choosing it now closes no doors.

## Consequences

### Positive

- **Portability:** the agent no longer depends on Firestore or GCP IAM for state.
- **Single integration point:** datastore/Dapr changes touch the controller only.
- **Reduced VM privileges:** the per-world VM SA can drop `roles/datastore.user` and the
  broad datastore scope.
- **Multi-world compatible:** the API boundary does not assume world-per-VM, keeping the
  door open to co-locating multiple worlds per host (and future bin-packing).
- **Smaller Dapr footprint:** the daprd-on-every-VM task (#259) is no longer needed.

### Negative / tradeoffs

- **Availability coupling:** status updates now require the controller to be reachable; a
  30s agent poll may cold-start Cloud Run. Acceptable given negligible cost impact.
- **Agent authentication required:** the agent needs a credential to call the API. Planned
  approach: a **per-server bearer token** injected through the existing `user-data`
  metadata channel (`internal/pulumi/programs/server.go:221`). (Security is not a driver,
  but the API cannot be fully unauthenticated.)
- **Self-stop must move:** VM self-stop (`cmd/machine-agent/main.go:296`) relocates to the
  controller, or behind a pluggable agent interface, since the agent should shed Compute
  API access along with datastore access.

## Sequencing with the Dapr Initiative (#252–259)

1. Implement (b): add controller endpoints + refactor the agent into an HTTP client.
2. **Drop #259** (daprd-as-systemd-on-VM) — it is obviated by (b).
3. Keep #256 (`DaprDB` adapter) and #257 (`DB_BACKEND` selection), now scoped to the
   controller only.

## Future Work

- Evaluate **GKE Autopilot with NodePort + world bin-packing** as a substrate (its own
  ADR), weighing the ~$10–12/month always-on control-plane cost against per-world VM
  savings at higher world counts.
- Revisit **world-per-VM vs multiple-worlds-per-host** independently of this decision.

## Related

- Issues: #252–259 (Dapr state-store abstraction), #259 (daprd-on-VM — to be dropped),
  #256 (`DaprDB` adapter), #257 (`DB_BACKEND` selection).
- Code: `cmd/machine-agent/main.go`, `internal/db/db.go`,
  `internal/handlers/servers/routes.go`, `internal/pulumi/programs/server.go`,
  `deploy/modules/gcp-cloud-run/controller.tf`.
