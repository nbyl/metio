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
