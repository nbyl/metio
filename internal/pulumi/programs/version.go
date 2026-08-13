package programs

// CurrentInfraVersion is the version of the Pulumi infrastructure program
// embedded in this controller binary. Bump this integer whenever the
// program code changes in a way that should trigger a re-deploy on
// existing servers. Pulumi's diff engine handles the actual migration.
const CurrentInfraVersion = 3
