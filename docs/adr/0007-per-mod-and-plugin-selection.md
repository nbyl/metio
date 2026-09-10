# ADR-0007: Per-Mod and Per-Plugin Selection

- **Status:** Proposed
- **Date:** 2026-09-10
- **Deciders:** Metio maintainers
- **Depends on:** ADR-0006 (platform adapters, search UI, configurable memory)

## Context

ADR-0006 lets a user install a *published pack*. It does not let them curate their own set -- add
one quality-of-life mod, try a plugin, remove it again. That is the iterative workflow, and it is
where ADR-0006's "every change recreates the VM" trade-off becomes untenable: recreation per
checkbox tick is not a product.

The upstream image provides the escape hatch.
[`MODS_FILE` / `PLUGINS_FILE`](https://docker-minecraft-server.readthedocs.io/en/latest/mods-and-plugins/#modplugin-url-listing-file)
accept **a URL to a text file** of one jar URL per line, re-read on every container start, with
entries removed from the list **automatically removed** from `mods/` and `plugins/`.

## Decision Drivers

1. **Mod changes must not recreate the VM.**
2. **Cover the platforms users actually use** -- Modrinth, CurseForge, Spigot, Hangar.
3. **Minimise new user-facing configuration** -- no loader taxonomy lesson.
4. Secrets server-side; additive persistence. (As ADR-0006.)

## Considered Options

- **A. Inline `MODS`/`PLUGINS` in the cloud-config.** Simplest; recreates the VM on every change.
  Rejected on driver 1.
- **B. `MODS_FILE`/`PLUGINS_FILE` -> a controller endpoint.** `user-data` holds a constant URL.
- **C. Machine-agent writes a local `mods.txt`.** Also avoids recreation and reuses the agent
  token, but adds a sync loop, a failure mode, and an ordering dependency between the agent and
  the Minecraft container.
- **D. Expose a "Server Type" field.** Maximum control, rejected on driver 3.
- **E. Infer the loader from selections.**

## Decision Outcome

Chosen: **B + E.**

### Resolved-list delivery

The cloud-config gains two **constant** placeholders:

```
MODS_FILE=${controllerUrl}/api/servers/${serverId}/mods.txt?token=${modsToken}
PLUGINS_FILE=${controllerUrl}/api/servers/${serverId}/plugins.txt?token=${modsToken}
```

All three components are fixed for a server's lifetime, so `user-data` is untouched by mod changes
and no recreation occurs. Metio acts as a **resolver**: a stored selection is translated at request
time into a concrete, version-pinned jar URL. Responses are `text/plain`, one URL per line.

Because the container cannot send headers, the token lives in the URL. A **separate, read-only,
rotatable per-server token** is minted rather than reusing `agentToken`, so a leak via access logs
exposes nothing but a list of public jar URLs.

### Loader inference

No Server Type field. The **first selection locks the loader** -- first plugin => `TYPE=PAPER`,
first mod => `TYPE=FABRIC`. Search then filters to items compatible with that loader and the
server's Minecraft version; incompatible items are never offered. Removing all items releases the
lock and returns the server to vanilla. The inferred loader is shown read-only, so the behaviour is
never invisible.

`TYPE` lives in the cloud-config, so vanilla => Paper/Fabric is `UpdateTypeRecreate`. This is a
**one-time** cost on the first item a server ever gets; every subsequent change is restart-only. We
considered serving `TYPE` via `LOAD_ENV_FROM_FILE` to make `user-data` fully static, but that
variable is sourced by `bash` in the container -- granting a network endpoint arbitrary shell
execution on the game server is not a trade we will make for a rare operation.

### Platform integration

| Platform | Content | Auth | Resolution target |
|---|---|---|---|
| Modrinth | mods + plugins | none | version file `url` |
| Spiget (SpigotMC) | plugins | none | `/resources/{id}/download` |
| Hangar (PaperMC) | plugins | none | version download URL |
| CurseForge | mods | **API key** | file `downloadUrl` |

Extends ADR-0006's proxy and cache. CurseForge remains optional and hidden without a key.

### Dependencies, compatibility, and applying changes

- Required dependencies are **auto-resolved where the platform exposes them** (in practice
  Modrinth), so picking a Fabric mod pulls in Fabric API without the user knowing it exists.
  Auto-added items are labelled as such. Platforms without dependency metadata install exactly what
  was picked.
- Every selection is pinned to a build compatible with the server's Minecraft version. If a version
  change would leave any selection with no compatible build, **the version change is blocked** and
  the offending items listed -- better a blocked update than a server that will not boot.
- Installed items surface an **update-available** indicator when a newer compatible build exists.
- Changing the selection persists immediately without disturbing a running server; a
  **"restart required"** banner with a Restart action lets the user choose when players are
  disconnected.

### Interaction with ADR-0006

A pack-driven server and a curated server are mutually exclusive modes. Switching between them is
classified `UpdateTypeRecreate`. The UI enforces this: when a pack is active, the per-mod
selection interface is disabled (and vice versa), with a clear message explaining why and how to
switch.

## Consequences

**Positive**

- Mod changes cost a restart, not a rebuild; `user-data` is stable across nearly all operations.
- Users never learn what a loader is.
- Persistence is additive JSON; existing servers unmarshal unchanged.

**Negative / risks**

- **Two new public endpoints** with token-in-URL auth. Mitigated by a dedicated read-only token and
  a trivial payload.
- **Four third-party API dependencies**; outages degrade search. Mitigated by cached, stale-tolerant
  reads.
- **Pinned jar URLs can rot**, and the container then fails to start. Must be surfaced clearly in
  status output.
- **First mod on a server causes one VM recreation** via the `TYPE` change.
- Interaction with ADR-0006 must be defined: a pack-driven server and a curated server are
  mutually exclusive modes, and switching between them is a recreation.
