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
4. **Secrets stay server-side.** CurseForge needs an API key that must not reach the browser.
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
- **D. Let the JVM size itself.** Drop `MAX_MEMORY` entirely, set
  `JVM_XX_OPTS=-XX:MaxRAMPercentage=...`, and cap the container with `--memory`. The heap tracks the
  machine automatically and **`user-data` need not encode any absolute number at all**, so machine
  type changes stay a resize.

### Pack installation

- **E. `MODPACK` (zip URL).** Generic but the user must find a raw zip; no metadata, no browsing.
- **F. Platform-native install** -- `MODRINTH_MODPACK` for `.mrpack`, `AUTO_CURSEFORGE` for
  CurseForge packs. The image resolves loader, Minecraft version, and mods from the pack manifest.
- **G. `GENERIC_PACK` + `LOAD_ENV_FROM_GENERIC_PACK`.** Most powerful, but sources shell-evaluated
  env files from a remote pack -- arbitrary code execution on the game server. Rejected.

## Decision Outcome

Chosen: **D for memory + F for packs.**

### Modpack installation

A server gains an optional pack reference on `db.ServerConfig`:

```go
Modpack *ModpackConfig `json:"modpack,omitempty"`
// Platform  string  // "modrinth" | "curseforge"
// ProjectID string
// VersionID string  // pinned; empty means latest
```

Rendered into the cloud-config as `MODRINTH_MODPACK` / `CF_PAGE_URL` + `AUTO_CURSEFORGE`
placeholders. `TYPE`, `VERSION` and the mod set all come from the pack manifest, so **Metio does
not ask the user for a loader or a Minecraft version** when a pack is selected -- the Minecraft
version field is disabled and shown as pack-controlled.

A pack is an **exclusive mode**: a server is either vanilla or pack-driven. Selecting, changing,
or removing a pack is a cloud-config change and is therefore classified `UpdateTypeRecreate`. This
is accepted -- changing modpacks is a rare, deliberate act that the user already expects to be
disruptive, and the data disk survives.

### Pack discovery UI

The controller proxies pack search behind an internal endpoint so the CurseForge key never
reaches the browser and CORS is avoided. Results from both platforms are normalised into one shape
(name, author, icon, downloads, supported Minecraft versions, loader). Results are cached with a
TTL and stale-on-outage fallback, mirroring `internal/services/minecraft_versions.go`.

CurseForge is **optional**: when no API key is configured, the CurseForge source is hidden from the
UI and skipped by the aggregator. Modrinth needs no key and always works.

### Memory

`MAX_MEMORY=3G` is removed from the cloud-config entirely. Instead:

- The Minecraft container is capped with `--memory` / `--memory-swap`, derived by the controller
  from `db.MachineTypes[machineType].MemoryGB` minus headroom for the OS, the `mc-backup` sidecar
  and the machine-agent.
- The JVM sizes its own heap inside that cap via `JVM_XX_OPTS=-XX:MaxRAMPercentage=...`.
- No absolute heap figure is encoded in `user-data`. Changing machine type remains
  `UpdateTypeResize` (`crud.go:397`) instead of being promoted to a full recreation.
- The effective heap is displayed read-only next to the machine type, so "bigger machine = bigger
  server" stays visible.

> Caveat worth validating during implementation: `USE_MEOWICE_FLAGS=true`
> (`server_cloud_config.yml:39`) may itself set `-Xmx`, which would override `MaxRAMPercentage`.
> Needs a check against the image's flag handling before this is finalised.

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
- Metio takes a dependency on Modrinth/CurseForge availability for *search* (not for boot).
- Large packs materially lengthen first boot; provisioning feedback must reflect that.
- `CurrentInfraVersion` (`internal/pulumi/programs/version.go`, currently `4`) must be bumped.
- Two copies of `buildProgramConfig` (`internal/handlers/servers/common.go:121` and
  `internal/handlers/tasks/handler.go:100`) must be kept in sync for every new field.
- The `minecraftVersion` field becomes conditionally meaningless, which complicates the wizard,
  the update modal, and backup source config (`internal/services/provisioning.go:451`).
