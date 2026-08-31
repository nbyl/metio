package programs

// CurrentInfraVersion is the version of the Pulumi infrastructure program
// embedded in this controller binary. Bump this integer whenever the
// program code changes in a way that should trigger a re-deploy on
// existing servers. Pulumi's diff engine handles the actual migration.
//
// Version 4 is a BREAKING change: the data disk mount relocates from
// /mnt/disks/minecraft/data to /mnt/disks/minecraft and world data moves into
// a data/ subdirectory alongside the new backup manifest directory. Existing
// servers must be migrated BEFORE re-deploying; see docs/DEPLOYMENT.md.
//
// Version 5 is a BREAKING change: the machine-agent container now runs with
// --privileged --pid=host and mounts the host data disk at /mnt/disks/minecraft
// so it can restore worlds by running restic inside the minecraft-backup
// container and control the host services via nsenter/systemd.
const CurrentInfraVersion = 5
