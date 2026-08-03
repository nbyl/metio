# ADR-0003: PostgreSQL State Backend (Cloud SQL or Bring-Your-Own)

- **Status:** Accepted
- **Date:** 2026-08-03
- **Deciders:** Metio maintainers

## Context

[ADR-0001](0001-controller-agent-api.md) established that only the Controller touches the
state store, and [ADR-0002](0002-dapr-state-store-key-schema-and-topology.md) introduced a
Dapr abstraction (`DaprDB`) with a flat key schema and a Cloud Run sidecar topology so the
backend becomes pluggable. ADR-0002 chose Firestore (Datastore mode) as the initial Dapr
backend.

Running Firestore through Dapr works but is GCP-locked and surfaced GCP-only friction
during the sidecar rollout (#255):

- `state.gcp.firestore` operates only in **Datastore mode**, requiring a **separate
  second Firestore database** alongside the existing Native-mode one.
- The component's `project_id` is not interpolated from environment variables; it had to
  be resolved via ADC auto-detection (`*detect-project-id*`) instead.

Meanwhile `DaprDB` (`internal/db/dapr.go`) uses only the store-agnostic surface that ADR-0002
relies on — single-key CRUD, bulk reads, and index keys. It makes **no use** of Firestore
queries, transactions, or Native-mode features. Swapping the backend is therefore a
component-level change with **zero application code changes**.

The `DB_BACKEND` toggle (#257) exists only to migrate safely. **It is transitional and will
be removed before release** — Dapr becomes the only datastore, and PostgreSQL becomes the
backend behind it.

This ADR is an **evolution** of the state-store design, not a rewrite: the Dapr key schema,
sidecar topology, and `DaprDB` adapter from ADR-0002 remain unchanged and continue to apply.

## Decision Drivers

In priority order (inherited from [ADR-0001](0001-controller-agent-api.md)):

1. **Provider cost** — must stay near $0 for an idle, single-user control plane.
2. **Cloud lock-in / portability** — reduce hard dependencies on GCP-specific services.
3. **Maintainability / architecture** — fewer integration points, clearer boundaries.
4. **End-user simplicity** — easy for a non-expert to self-host.
5. **No application code churn** — the chosen backend must work through the existing
   `DaprDB` adapter unchanged.

## Considered Options

### (a) Stay on Firestore (Datastore mode) behind Dapr

Keep `state.gcp.firestore` as the backend.

- Cost: ~$0/month idle (unchanged).
- Portability: **poor** — still GCP-locked; keeps the dual-database (Native + Datastore)
  requirement and the ADC quirks seen in #255.
- Maintainability: Firestore index/rules management remains; the Datastore-mode
  component is a niche Dapr integration.
- Simplicity: single-account, zero external services — but the backend is not portable.

### (b) PostgreSQL via Dapr — Cloud SQL or bring-your-own *(chosen)*

Move to `state.postgresql` behind the existing Dapr abstraction, selectable at deploy time
through two OpenTofu topologies (see [Decision Outcome](#decision-outcome)).

- Cost: Cloud SQL is **always-on** (~$7–9+/month, no scale-to-zero); bring-your-own
  Postgres (e.g. Neon, CockroachDB) has free tiers that **scale to zero**, preserving the
  ~$0-idle goal for cost-sensitive users.
- Portability: **best** — PostgreSQL is a ubiquitous, provider-neutral database; the same
  component and connection string work with Cloud SQL, Neon, CockroachDB, or any other
  Postgres.
- Maintainability: one Postgres instance replaces two Firestore databases; no Firestore
  index/rules management; JSONB payloads are directly queryable.
- Simplicity: two supported paths — one fully GCP-managed, one fully user-supplied.

### (c) Other Dapr state stores (Redis/Valkey, MySQL)

Rejected: Redis/Valkey would reintroduce a paid always-on component or an unmanaged
self-host burden, and MySQL is less universally available as a managed free-tier offering
than Postgres. Neither improves on Postgres for the drivers above.

## Decision Outcome

**Chosen option: (b) PostgreSQL via Dapr**, with two deploy-time topologies.

The baked `statestore` component becomes `state.postgresql`. Because `DaprDB` is
store-agnostic (CRUD + bulk + index keys only), **no Go code changes** are required —
`internal/db/dapr.go` stays as-is, and the ADR-0002 key schema carries over unchanged
(rows store the flat Dapr keys as `TEXT PRIMARY KEY` with `JSONB` values).

The OpenTofu module selects the topology with a module variable (e.g. `postgres_mode`):

### 1. `cloudsql` — auto-provisioned Cloud SQL

The module provisions `google_sql_database_instance` (+ database and user), builds the
connection string, and stores it in a Secret Manager secret consumed by the component via
`secretKeyRef`.

- Uses the **default connectivity**: Cloud SQL public IP with TLS (`sslmode=require`).
  Private IP / VPC is an optional hardening path, not the default.
- Breaks the scale-to-zero paradigm **by design**: a Cloud SQL instance is always on.
  This is intended for users who prefer **single-account simplicity over cost** — one GCP
  project, no external database account, everything billed on the same project.

### 2. `byo` — bring-your-own Postgres

The user supplies a PostgreSQL connection string (Neon, CockroachDB, or any Postgres);
the module writes it to the Secret Manager secret and provisions **no** database.

- Uses the provider's TLS endpoint as given.
- **Recommended for cost-sensitive users, especially self-hosters**: free tiers scale to
  zero, keeping the control plane near $0/month idle in line with ADR-0001.

### Cost and scale-to-zero summary

| Backend | Approximate idle cost | Scales to zero | Portability | Notes |
|---|---|---|---|---|
| Firestore (Datastore mode) | ~$0 | Yes | GCP-only | Dual database + ADC quirks (#255) |
| Cloud SQL | ~$7–9+/month | **No** | Postgres | Single-account simplicity; always-on |
| BYO Postgres (Neon / CockroachDB) | ~$0 (free tier) | Yes | Postgres | Recommended for cost-sensitive / self-host |

### Component design

```yaml
# statestore.yaml — baked into the daprd image
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
  name: statestore
spec:
  type: state.postgresql
  version: v1
  metadata:
    - name: connectionString
      secretKeyRef:
        name: postgres-connection-string
        key: value
    - name: tableName
      value: state
  auth:
    secretStore: local-secret-file
```

The connection string lives in Secret Manager (matching the existing secret-handling
pattern in `controller.tf`) and is surfaced to daprd through the local secret-file store,
keeping the component self-contained and store-agnostic.

## Consequences

### Positive

- **Portability:** the backend is now provider-neutral — Cloud SQL, Neon, CockroachDB, or
  any Postgres — with no application code changes.
- **Single database:** one Postgres instance replaces the dual Firestore databases and
  their index/rules management.
- **Ubiquitous tooling:** standard Postgres clients, migrations, and debugging work out of
  the box; JSONB payloads are directly queryable for future reporting.
- **Explicit cost model:** users choose between Cloud SQL's single-account simplicity and
  a free-tier BYO Postgres that preserves scale-to-zero.

### Negative / tradeoffs

- **Cloud SQL breaks scale-to-zero:** accepted and intentional for the simplicity cohort;
  the BYO topology restores ~$0-idle for cost-sensitive users.
- **Connection-string secret management:** the credential lives in Secret Manager and must
  be rotated; BYO users manage their provider account and credentials separately.
- **Local development changes:** the local dev flow moves from the Datastore emulator to a
  local Postgres container (`dapr/components/statestore-local.yaml`, Makefile dev targets).

### Implementation impact

A follow-up implementation will touch infrastructure and dev tooling only:

- `deploy/daprd/components/statestore.yaml` — switch to `state.postgresql`.
- New `deploy/modules/gcp-cloud-run/postgres.tf` — Cloud SQL resources and the
  connection-string secret.
- `deploy/variables.tf` + module `variables.tf` — `postgres_mode` and the BYO connection
  string input.
- `dapr/components/statestore-local.yaml` + Makefile dev targets — local Postgres container.

No change to `internal/db/dapr.go`; `internal/config/config.go` is unaffected beyond the
eventual removal of the `DB_BACKEND` toggle. The `DB_BACKEND` toggle is retained during
migration and **removed before release** — Dapr becomes the only datastore.

## Migration

#253 is re-targeted from Firestore-Native → Firestore-Datastore to
Firestore-Native → PostgreSQL. The ADR-0002 deterministic key mapping is unchanged; the
migration job reads each Firestore document, constructs the target Dapr key, and writes it
via the Dapr State API. Rollback during the migration window uses the temporary
`DB_BACKEND` toggle; the Native Firestore database is removed by #254 once migration is
complete.

## Related

- ADR-0001 — Controller-mediated agent state access (gates this work)
- ADR-0002 — Dapr state store key schema and Cloud Run sidecar topology (foundation;
  unchanged)
- Issues: #252–259 (Dapr State Management Migration), #253 (data migration — re-targeted
  to PostgreSQL), #254 (remove Firestore)
