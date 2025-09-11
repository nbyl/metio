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
    }
  }
}
