# Central backup infrastructure (ADR-0004)
#
# One deployment-wide GCS bucket holds every server's Restic repository under
# per-server prefixes (servers/{server-id}/restic). The bucket must never be
# destroyed while backups exist, so it deliberately has no force_destroy and no
# lifecycle rule is used as the primary retention mechanism. Retention is driven
# by the controller (per-server active retention + deployment-level deleted-server
# retention); lifecycle rules may only be added as an optional coarse safety net.

resource "google_storage_bucket" "backups" {
  name                        = "${var.project_id}-${var.environment}-backups"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  versioning {
    enabled = true
  }
}

# Deployment-wide Restic password. It grants access to every server repository,
# a deliberate simplicity tradeoff documented in ADR-0004. Rotate by adding a new
# secret version; the controller reads "latest".
resource "random_password" "backup_restic_password" {
  length  = 32
  special = false
}

resource "google_secret_manager_secret" "backup_restic_password" {
  secret_id = "${var.environment}-backup-restic-password"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "backup_restic_password_value" {
  secret      = google_secret_manager_secret.backup_restic_password.id
  secret_data = random_password.backup_restic_password.result
}

# The controller reads the password to bake it into each server's cloud-config
# (RESTIC_PASSWORD), so it needs access to the secret.
resource "google_secret_manager_secret_iam_member" "secret-access-backup_restic_password" {
  secret_id  = google_secret_manager_secret.backup_restic_password.id
  role       = "roles/secretmanager.secretAccessor"
  member     = "serviceAccount:${google_service_account.controller_service_account.email}"
  depends_on = [google_secret_manager_secret.backup_restic_password]
}

# The controller manages backup objects (e.g. cleanup of expired deleted-server
# repositories) — scoped to the central bucket instead of the whole project.
resource "google_storage_bucket_iam_member" "controller_backup_admin" {
  bucket = google_storage_bucket.backups.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.controller_service_account.email}"
}

# GCS grants storage.objects.list at the bucket level; IAM conditions cannot
# scope it to a prefix (for list calls resource.name resolves to the bucket,
# and storage.googleapis.com/objectListPrefix is only supported in Credential
# Access Boundaries). Restic needs list on every init/snapshots/prune, so each
# server VM service account gets this bucket-wide list-only custom role while
# object get/create/delete stay scoped to its own prefix by the conditional
# binding in the Pulumi server program. Role name must match
# backupObjectListRoleID() in internal/pulumi/programs/server.go.
resource "google_project_iam_custom_role" "backup-object-list" {
  role_id     = "${replace(var.environment, "-", "_")}_backup_object_list"
  title       = "List backup objects for ${var.environment}"
  description = "Grants only storage.objects.list on the central backup bucket so Restic can enumerate a per-server repository. Bucket-wide because GCS list cannot be prefix-scoped by IAM conditions."
  permissions = [
    "storage.objects.list",
  ]
}

# Per-server VM service accounts get prefix-scoped object access to the central
# bucket from the Pulumi server program (least privilege), not here, because
# those service accounts are created per server.
