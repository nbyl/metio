# spike-k3s

> **SPIKE ONLY — delete this directory at the end of milestone #10.**
>
> Tracked by [#548](https://github.com/nbyl/metio/issues/548), whose acceptance
> criteria include removing this harness.

Throwaway harness for the [ADR-0008](../../docs/adr/0008-kubernetes-runtime.md)
Kubernetes feasibility spike ([#542](https://github.com/nbyl/metio/issues/542)).

It creates and destroys a single GCE VM shaped like a real Metio server, so each
k3s experiment starts from a known-clean machine without hand-typed `gcloud`
commands.

This is intentionally crude. It exists for about a week.

## Scope

Per the milestone ground rules, this harness **does not touch
`internal/pulumi/`**. Modifying the production provisioning path would turn a
spike into a migration. Nothing here is intended to survive the spike.

## Usage

```sh
cd tools/spike-k3s

./spike.sh create --os cos          # stand up a COS VM
./spike.sh ssh                      # log in
./spike.sh list                     # show what exists (leak check)
./spike.sh destroy                  # remove everything
```

| Flag | Default | Notes |
|---|---|---|
| `--os cos\|flatcar\|ubuntu` | `cos` | See image table below |
| `--machine-type` | `e2-medium` | 2 vCPU / 4 GB, present in `db.MachineTypes` |
| `--zone` | `europe-west3-b` | Matches the Makefile's `LOCATION` default |
| `--disk-size` | `20` | Matches the setup wizard's `INITIAL_FORM` |
| `--user-data FILE` | none | cloud-init for cos/ubuntu, Ignition for flatcar |
| `--no-spot` | off | **Debugging only**, see below |

## What it creates

All resources are prefixed `spike-k3s`, labelled `purpose=adr0008-spike` and
tagged `spike-k3s`, so strays are easy to find with `./spike.sh list`.

| Resource | Config | Mirrors |
|---|---|---|
| `spike-k3s-addr` | static regional IP | `server.go:348` |
| `spike-k3s-fw` | icmp + tcp:25565 from `0.0.0.0/0`, target tag | `server.go:355` |
| `spike-k3s-data` | `pd-standard`, attached as `minecraft-data` | `server.go:327` |
| `spike-k3s` | SPOT, no restart on failure, `TERMINATE` on maintenance | `server.go:407-409` |

The disk is attached with `--device-name minecraft-data` so it appears at
`/dev/disk/by-id/google-minecraft-data`, exactly as
`server_cloud_config.yml:2` expects. Mount logic written during the spike
therefore ports to production unchanged.

## Deliberate design choices

**The data disk is attached raw and is not formatted or mounted.** Formatting
differs per OS — cloud-init `fs_setup` for COS and Ubuntu, Ignition for Flatcar
— so it belongs to [#543](https://github.com/nbyl/metio/issues/543) rather than
here. This keeps the harness OS-agnostic.

**Default compute service account, not a dedicated one.** Production creates a
per-server service account with `cloud-platform` scope
(`server.go:243-268`) because the machine-agent calls GCP APIs. The spike pulls
only public images and needs none of that.

**Port 6443 is not exposed.** The k3s API server is reached by SSHing in and
running `kubectl` locally. Opening cluster admin to `0.0.0.0/0` for a
convenience that SSH already provides is not a trade worth making, even for a
week.

**`--no-spot` exists but is restricted.** It is an escape hatch for SPOT
capacity failures while doing work where scheduling is irrelevant. It must
**not** be used for [#545](https://github.com/nbyl/metio/issues/545), which
tests preemption recovery and therefore needs genuine SPOT semantics.

## OS images

Verified against the live API on 2026-09-11:

| `--os` | Image project | Family | Resolved at time of writing |
|---|---|---|---|
| `cos` | `cos-cloud` | `cos-stable` | `cos-stable-121-18867-584-7` |
| `flatcar` | `kinvolk-public` | `flatcar-stable` | `flatcar-stable-4593-2-5` |
| `ubuntu` | `ubuntu-os-cloud` | `ubuntu-2404-lts-amd64` | `ubuntu-2404-noble-amd64-v20260906` |

> **Flatcar caveat.** Images in `kinvolk-public` can be *used* but not *listed*
> by an ordinary account: `gcloud compute images list --project kinvolk-public`
> fails with a permissions error, while `describe-from-family` succeeds and
> instance creation works. Do not add list-based validation of image families —
> it will fail misleadingly for Flatcar only.

## Cost

SPOT `e2-medium` instances are cheap, but a forgotten VM bills silently.
`./spike.sh destroy` is idempotent and deletes in dependency order; always
finish with `./spike.sh list` and confirm it is empty.
