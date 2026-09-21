package programs

// CurrentInfraVersion is the version of the Pulumi infrastructure program
// embedded in this controller binary. Bump this integer whenever the
// program code changes in a way that should trigger a re-deploy on
// existing servers. Pulumi's diff engine handles the actual migration.
//
// Version 4 was a BREAKING change: the data disk mount relocates from
// /mnt/disks/minecraft/data to /mnt/disks/minecraft and world data moves into
// a data/ subdirectory alongside the new backup manifest directory. Existing
// servers must be migrated BEFORE re-deploying; see docs/DEPLOYMENT.md.
//
// Version 5 sizes the JVM heap as a percentage of host RAM instead of the
// hardcoded 3G: user-data gains MAX_MEMORY=75%, which implements
// -XX:MaxRAMPercentage. The percentage keeps user-data byte-identical across
// machine types, so machine-type changes remain a resize rather than a
// recreate.
//
// Version 6 adds modpack support (ADR-0006): a pack-driven server passes
// MODRINTH_MODPACK instead of VERSION, and the Minecraft version becomes
// pack-controlled. Those servers deploy with an empty minecraftVersion.
const CurrentInfraVersion = 6
