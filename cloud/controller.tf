resource "google_cloud_run_v2_service" "controller" {
  name                = "${var.environment}-controller"
  location            = var.region
  deletion_protection = false
  ingress             = "INGRESS_TRAFFIC_ALL"

  template {
    containers {
      image = var.controller_image
      env {
        name  = "INSTANCE_NAME"
        value = google_compute_instance.minecraft-server.name
      }
      env {
        name  = "GCP_ZONE"
        value = var.zone
      }
      env {
        name  = "GCP_PROJECT"
        value = var.project_id
      }
      env {
        name  = "ALLOWED_USERS"
        value = join(" ", var.allowed_users)
      }
    }
  }
}

resource "google_cloud_run_service_iam_binding" "default" {
  location = google_cloud_run_v2_service.controller.location
  service  = google_cloud_run_v2_service.controller.name
  role     = "roles/run.invoker"
  members = [
    "allUsers"
  ]
}
