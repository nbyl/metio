# ADR-0006: Modpack Support and Configurable Server Memory

- **Status:** Accepted
- **Date:** 2026-09-10
- **Deciders:** Metio maintainers

## Context

Every Metio server is implicitly vanilla. `internal/pulumi/programs/server_cloud_config.yml`
runs `itzg/minecraft-server:stable-java25` with no `TYPE` variable (lines 32, 42), so the image
defaults to `TYPE=VANILLA`. There is no mod, plugin, or modpack code anywhere in the repository.

Two things block modded servers, and they are entangled:

**1. No way to install a pack.** `db.ServerConfig` (`internal/db/server_config.go:69`) has no
field for one, and the cloud-config sets no pack variables.

**2. Memory is hardcoded.** `server_cloud_config.yml:38` sets `MAX_MEMORY=3G` on every server
regardless of machine type. A user on `n2-highmem-8` (64 GB) gets 3 GB; a user on `e2-small`
(2 GB) gets a JVM configured for more RAM than the box has. Modpacks routinely need 6-10 GB, so
pack support without addressing memory ships a feature that OOMs on arrival.

`db.MachineTypes` (`internal/db/constants.go:9`) already records `MemoryGB` for all 31 supported
machine types, so the information needed to size the heap correctly is already present.

The governing constraint is `internal/pulumi/programs/server.go:428`:
`pulumi.ReplaceOnChanges([]string{"metadata.user-data"})` with `DeleteBeforeReplace(true)` --
**any cloud-config change destroys and recreates the VM.**

## Decision Drivers

1. **Deliver a modded server with the fewest possible user decisions.** A pack declares its own
   loader, Minecraft version and mod set; the user should pick a pack and nothing else.
2. **The JVM heap must fit the machine.** Correct by construction, not by user arithmetic.
3. **Minimise recreations.** Recreation is acceptable for rare structural changes, not routine ones.
4. **No third-party credentials on the game server.** Instance metadata (`user-data`) is readable
   by anyone with instance access or metadata-server reach, so a pack source that requires a
   credential inside the container is materially worse than one that does not.
5. **Additive persistence.** Dapr JSON state, no migrations (ADR-0002/0003); new fields must
   default safely for existing servers.

## Considered Options

### Memory sizing

- **A. Keep it hardcoded.** Rejected; blocks the entire feature.
- **B. New user-set `memoryGB` field.** Full control, but adds a wizard field, needs validation
  against machine RAM, and every change rewrites `user-data` -> recreation (driver 3).
- **C. Auto-derive from the machine type.** Controller computes `MAX_MEMORY` from
  `db.MachineTypes[mt].MemoryGB`, reserving headroom for the OS, backup sidecar and machine-agent.
  No new user-facing field. Memory then changes only when machine type changes -- which today is
  classified `UpdateTypeResize` (`crud.go:397`) and would be promoted to a recreation.
- **D. Size the heap as a percentage of machine RAM.** `MAX_MEMORY` accepts a `<size>%` value,
  which the image implements as `-XX:MaxRAMPercentage`. The heap tracks the machine automatically
  and **`user-data` encodes no absolute number at all**, so machine type changes stay a resize.

### Pack installation

- **E. `MODPACK` (zip URL).** Generic but the user must find a raw zip; no metadata, no browsing.
- **F. `MODRINTH_MODPACK`.** Platform-native `.mrpack` install. The image resolves loader,
  Minecraft version, and mods from the pack manifest. Requires **no credentials**.
- **G. `AUTO_CURSEFORGE`.** Equivalent coverage for CurseForge packs, and the larger catalogue.
  **Rejected on driver 4:** the pack install runs *inside the container*, so `CF_API_KEY` must be
  present on the VM and would therefore land in instance metadata. That is a meaningfully
  different exposure from a key held only by the controller, and it is not justified by a
  second pack source. CurseForge may still be reconsidered for ADR-0007, where per-mod resolution
  happens controller-side and the key never leaves it.
- **H. `GENERIC_PACK` + `LOAD_ENV_FROM_GENERIC_PACK`.** Most powerful, but sources shell-evaluated
  env files from a remote pack -- arbitrary code execution on the game server. Rejected.

## Decision Outcome

Chosen: **D for memory + F for packs.** Modpack support is **Modrinth-only**.

### Modpack installation

A server gains an optional pack reference on `db.ServerConfig`:

```go
Modpack *ModpackConfig `json:"modpack,omitempty"`
// Platform  string  // "modrinth" (reserved for future sources)
// ProjectID string
// VersionID string  // pinned; empty means latest
```

Rendered into the cloud-config as a `MODRINTH_MODPACK` placeholder. `TYPE`, `VERSION` and the mod
set all come from the pack manifest, so **Metio does not ask the user for a loader or a Minecraft
version** when a pack is selected -- the Minecraft version field is disabled and shown as
pack-controlled.

The `Platform` field is retained despite having a single valid value today, so that adding a
second source later is an additive change rather than a schema migration.

A pack is an **exclusive mode**: a server is either vanilla or pack-driven. Selecting, changing,
or removing a pack is a cloud-config change and is therefore classified `UpdateTypeRecreate`. This
is accepted -- changing modpacks is a rare, deliberate act that the user already expects to be
disruptive, and the data disk survives.

### Pack discovery UI

The controller proxies pack search behind an internal endpoint rather than having the browser call
Modrinth directly, so that CORS is avoided, results can be cached and normalised, and a future
credentialed source can be added without touching the frontend. Results are cached with a TTL and
stale-on-outage fallback, mirroring `internal/services/minecraft_versions.go`.

Modrinth's API requires no key, so pack search has no configuration and is always available.

### Memory

The hardcoded `MAX_MEMORY=3G` is replaced with a percentage value. The itzg image accepts
`MAX_MEMORY=<size>%` and implements it as `-XX:MaxRAMPercentage`, so the heap is expressed
relative to the machine rather than as an absolute figure:

- Because these are dedicated single-purpose VMs, a percentage of host RAM is the correct model;
  the reserved remainder covers the OS, the `mc-backup` sidecar and the machine-agent.
- `user-data` stays byte-identical across machine types, so changing machine type remains
  `UpdateTypeResize` (`crud.go:397`) rather than being promoted to a full recreation.
- No container `--memory` cap and no boot-time computation are required. One constant replaces
  another.
- The effective heap is displayed read-only next to the machine type, so "bigger machine = bigger
  server" stays visible.

`USE_MEOWICE_FLAGS=true` (`server_cloud_config.yml:39`) was checked and does **not** set `-Xmx`;
MeowIce's flags are GC and JIT tuning flags derived from Aikar's, and heap sizing remains solely
under `MEMORY` / `INIT_MEMORY` / `MAX_MEMORY`.

> Minor open point for implementation: Aikar/MeowIce flag tuning branches on `MEMORY >= 12G`, and
> that comparison may not evaluate a percentage value. This affects GC tuning only, not
> correctness, but should be confirmed empirically.

### UI design process

The interface is prototyped in **v0** by importing the existing project for visual context, then
hand-ported onto our shadcn/ui components per ADR-0005. This is a working practice, not an
architectural commitment; no v0-generated code ships unreviewed.

## Consequences

**Positive**

- One click gets a user a fully working modded server, with no Minecraft ecosystem knowledge.
- Heap size becomes correct on all 31 machine types instead of wrong on 30 of them.
- The Modrinth adapter, the search/select UI, and memory sizing are all reused by ADR-0007.
- No new public endpoints and no new tokens; the attack surface is unchanged.

**Negative / risks**

- Changing or removing a pack recreates the VM.
- Metio takes a dependency on Modrinth availability for *search* (not for boot).
- **CurseForge packs are not supported**, which excludes a large share of the popular pack
  catalogue. This is the main functional cost of the decision and the most likely reason to
  revisit this ADR.
- Large packs materially lengthen first boot; provisioning feedback must reflect that.
- `CurrentInfraVersion` (`internal/pulumi/programs/version.go`, currently `4`) must be bumped.
- Two copies of `buildProgramConfig` (`internal/handlers/servers/common.go:121` and
  `internal/handlers/tasks/handler.go:100`) must be kept in sync for every new field.
- The `minecraftVersion` field becomes conditionally meaningless, which complicates the wizard,
  the update modal, and backup source config (`internal/services/provisioning.go:451`).
