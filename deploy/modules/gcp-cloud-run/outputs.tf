output "controller_uri" {
  description = "The URI of the deployed Cloud Run controller service."
  value       = google_cloud_run_v2_service.controller.uri
}

output "controller_service_account_email" {
  description = "The email of the controller service account."
  value       = google_service_account.controller_service_account.email
}

output "pulumi_state_bucket" {
  description = "The name of the GCS bucket for Pulumi state."
  value       = google_storage_bucket.pulumi-state.name
}

output "cloud_tasks_queue_name" {
  description = "The name of the Cloud Tasks queue for provisioning."
  value       = google_cloud_tasks_queue.provisioning.name
}

output "backup_bucket" {
  description = "The deployment-wide central backup bucket (ADR-0004)."
  value       = google_storage_bucket.backups.name
}

output "backup_restic_password_secret_id" {
  description = "Secret Manager secret ID holding the deployment-wide Restic password."
  value       = google_secret_manager_secret.backup_restic_password.secret_id
}
