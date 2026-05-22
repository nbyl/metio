resource "google_project_iam_custom_role" "controller-role" {
  role_id = "${replace(var.environment, "-", "_")}_controller"
  title   = "Controller for ${var.environment}"
  permissions = [
    "artifactregistry.repositories.deleteArtifacts",
    "artifactregistry.repositories.downloadArtifacts",
    "artifactregistry.repositories.uploadArtifacts",
    "compute.instances.get",
    "compute.instances.start",
    "compute.instances.stop",
    "compute.zoneOperations.get",
    "datastore.entities.allocateIds",
    "datastore.entities.create",
    "datastore.entities.delete",
    "datastore.entities.get",
    "datastore.entities.list",
    "datastore.entities.update",
    "iam.serviceAccounts.signBlob",
    "logging.logEntries.create",
    "monitoring.timeSeries.create",
    "serviceusage.services.get",
    "serviceusage.services.use",
    "telemetry.traces.write",
  ]
}

resource "google_service_account" "controller_service_account" {
  account_id = "${var.environment}-c-sa"
}

resource "google_project_iam_binding" "controller-role-binding" {
  project = var.project_id
  role    = "projects/${var.project_id}/roles/${google_project_iam_custom_role.controller-role.role_id}"
  members = [
    "serviceAccount:${google_service_account.controller_service_account.email}"
  ]
}

# Grant the controller service account the ability to create Firebase custom tokens
# This allows it to sign tokens using its own identity
resource "google_service_account_iam_member" "controller_token_creator" {
  service_account_id = google_service_account.controller_service_account.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.controller_service_account.email}"
}

resource "google_secret_manager_secret" "client_id" {
  secret_id = "${var.environment}-client_id"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "client_id_dummy" {
  secret                 = google_secret_manager_secret.client_id.id
  secret_data_wo_version = 0
  secret_data            = "dummy"
}

resource "google_secret_manager_secret" "client_secret" {
  secret_id = "${var.environment}-client_secret"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "client_secret_dummy" {
  secret                 = google_secret_manager_secret.client_secret.id
  secret_data_wo_version = 0
  secret_data            = "dummy"
}

resource "google_secret_manager_secret" "base_url" {
  secret_id = "${var.environment}-base_url"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "base_url_dummy" {
  secret                 = google_secret_manager_secret.base_url.id
  secret_data_wo_version = 0
  secret_data            = "http://dummy:3000"
}

resource "google_cloud_run_v2_service" "controller" {
  name                = "${var.environment}-controller"
  location            = var.region
  deletion_protection = false
  ingress             = "INGRESS_TRAFFIC_ALL"

  scaling {
    manual_instance_count = 0
    min_instance_count    = 0
  }

  template {
    service_account = google_service_account.controller_service_account.email

    containers {
      image = var.controller_image
      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }
      env {
        name  = "REGION"
        value = var.region
      }
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
        value = var.admin_users
      }
      env {
        name  = "SESSION_KEY"
        value = "session-key-${var.environment}-${random_id.default.hex}"
      }
      env {
        name = "GOOGLE_CLIENT_ID"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.client_id.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "GOOGLE_CLIENT_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.client_secret.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "BASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.base_url.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "FIREBASE_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.firebase_api_key.secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "PULUMI_STATE_BUCKET"
        value = google_storage_bucket.pulumi-state.name
      }

    }
  }
}

# TODO: SESSION_KEY

resource "google_cloud_run_service_iam_binding" "default" {
  location = google_cloud_run_v2_service.controller.location
  service  = google_cloud_run_v2_service.controller.name
  role     = "roles/run.invoker"
  members = [
    "allUsers"
  ]
}

resource "google_secret_manager_secret_iam_member" "secret-access-client_id" {
  secret_id  = google_secret_manager_secret.client_id.id
  role       = "roles/secretmanager.secretAccessor"
  member     = "serviceAccount:${google_service_account.controller_service_account.email}"
  depends_on = [google_secret_manager_secret.client_id]
}

resource "google_secret_manager_secret_iam_member" "secret-access-client_secret" {
  secret_id  = google_secret_manager_secret.client_secret.id
  role       = "roles/secretmanager.secretAccessor"
  member     = "serviceAccount:${google_service_account.controller_service_account.email}"
  depends_on = [google_secret_manager_secret.client_secret]
}

resource "google_secret_manager_secret_iam_member" "secret-access-base_url" {
  secret_id  = google_secret_manager_secret.base_url.id
  role       = "roles/secretmanager.secretAccessor"
  member     = "serviceAccount:${google_service_account.controller_service_account.email}"
  depends_on = [google_secret_manager_secret.base_url]
}

resource "google_secret_manager_secret" "firebase_api_key" {
  secret_id = "${var.environment}-firebase_api_key"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "firebase_api_key_dummy" {
  secret                 = google_secret_manager_secret.firebase_api_key.id
  secret_data_wo_version = 0
  secret_data            = "dummy"
}

resource "google_secret_manager_secret_iam_member" "secret-access-firebase_api_key" {
  secret_id  = google_secret_manager_secret.firebase_api_key.id
  role       = "roles/secretmanager.secretAccessor"
  member     = "serviceAccount:${google_service_account.controller_service_account.email}"
  depends_on = [google_secret_manager_secret.firebase_api_key]
}

resource "google_project_iam_member" "sa_storage_object_admin" {
  project = var.project_id
  role    = "roles/storage.objectAdmin"
  member  = "serviceAccount:${google_service_account.controller_service_account.email}"
}

resource "google_project_iam_member" "sa_storage_admin" {
  project = var.project_id
  role    = "roles/storage.admin"
  member  = "serviceAccount:${google_service_account.controller_service_account.email}"
}
