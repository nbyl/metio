resource "google_cloud_scheduler_job" "backup_cleanup" {
  name             = "${var.environment}-backup-cleanup"
  description      = "Periodically triggers the controller's expired deleted-server backup cleanup sweep (ADR-0004)."
  region           = var.region
  schedule         = var.backup_cleanup_schedule
  time_zone        = "Etc/UTC"
  attempt_deadline = "600s"
  paused           = false

  retry_config {
    retry_count = 2
  }

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_v2_service.controller.uri}/api/backups/cleanup"

    oidc_token {
      service_account_email = google_service_account.controller_service_account.email
      audience              = google_cloud_run_v2_service.controller.uri
    }
  }

  depends_on = [
    google_cloud_run_service_iam_binding.default,
    google_storage_bucket_iam_member.controller_backup_admin,
  ]
}
