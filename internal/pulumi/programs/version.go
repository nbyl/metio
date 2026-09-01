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
const CurrentInfraVersion = 4
