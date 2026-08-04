resource "google_cloud_tasks_queue" "provisioning" {
  name     = "${var.environment}-metio-provisioning"
  location = var.region

  rate_limits {
    max_concurrent_dispatches = 10
    max_dispatches_per_second = 10
  }

  retry_config {
    max_attempts  = 3
    min_backoff   = "120s"
    max_backoff   = "600s"
    max_doublings = 4
  }
}

resource "google_project_iam_custom_role" "controller-role" {
  role_id = "${replace(var.environment, "-", "_")}_controller"
  title   = "Controller for ${var.environment}"
  permissions = [
    "artifactregistry.repositories.deleteArtifacts",
    "artifactregistry.repositories.downloadArtifacts",
    "artifactregistry.repositories.uploadArtifacts",
    "cloudtasks.tasks.create",
    "cloudtasks.tasks.get",
    "compute.addresses.create",
    "compute.addresses.delete",
    "compute.addresses.get",
    "compute.addresses.setLabels",
    "compute.addresses.use",
    "compute.disks.create",
    "compute.disks.delete",
    "compute.disks.get",
    "compute.disks.resize",
    "compute.disks.setLabels",
    "compute.disks.use",
    "compute.firewalls.create",
    "compute.firewalls.delete",
    "compute.firewalls.get",
    "compute.instances.addResourcePolicies",
    "compute.instances.attachDisk",
    "compute.instances.create",
    "compute.instances.delete",
    "compute.instances.get",
    "compute.instances.removeResourcePolicies",
    "compute.instances.setLabels",
    "compute.instances.setMachineType",
    "compute.instances.setMetadata",
    "compute.instances.setScheduling",
    "compute.instances.setServiceAccount",
    "compute.instances.setTags",
    "compute.instances.start",
    "compute.instances.stop",
    "compute.instances.update",
    "compute.instances.use",
    "compute.networks.get",
    "compute.networks.updatePolicy",
    "compute.resourcePolicies.create",
    "compute.resourcePolicies.delete",
    "compute.resourcePolicies.get",
    "compute.resourcePolicies.use",
    "compute.subnetworks.use",
    "compute.subnetworks.useExternalIp",
    "compute.zoneOperations.get",
    "compute.zones.get",
    "datastore.entities.allocateIds",
    "datastore.entities.create",
    "datastore.entities.delete",
    "datastore.entities.get",
    "datastore.entities.list",
    "datastore.entities.update",
    "iam.serviceAccounts.actAs",
    "iam.serviceAccounts.create",
    "iam.serviceAccounts.delete",
    "iam.serviceAccounts.get",
    "iam.serviceAccounts.signBlob",
    "logging.logEntries.create",
    "monitoring.timeSeries.create",
    "resourcemanager.projects.getIamPolicy",
    "resourcemanager.projects.setIamPolicy",
    "serviceusage.services.enable",
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

resource "google_service_account_iam_member" "controller_actas_self" {
  service_account_id = google_service_account.controller_service_account.name
  role               = "roles/iam.serviceAccountUser"
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
  secret_data_wo         = "dummy"
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
  secret_data_wo         = "dummy"
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
  secret_data_wo         = "http://dummy:3000"
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
    timeout         = "1800s"

    scaling {
      max_instance_count = 1
    }

    containers {
      name       = "controller"
      image      = var.controller_image
      depends_on = ["daprd"]

      ports {
        container_port = 8080
      }

      startup_probe {
        initial_delay_seconds = 10
        timeout_seconds       = 3
        period_seconds        = 3
        failure_threshold     = 20

        http_get {
          path = "/healthz"
          port = 8080
        }
      }

      liveness_probe {
        http_get {
          path = "/healthz"
          port = 8080
        }
        initial_delay_seconds = 10
        period_seconds        = 30
        timeout_seconds       = 5
        failure_threshold     = 3
      }
      resources {
        limits = {
          cpu    = "1000m"
          memory = "1Gi"
        }
        startup_cpu_boost = true
      }
      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }
      env {
        name  = "REGION"
        value = var.region
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
      env {
        name  = "MACHINE_AGENT_IMAGE"
        value = var.machine_agent_image
      }
      env {
        name  = "OPERATION_MODE"
        value = "cloudtasks"
      }
      env {
        name  = "CLOUD_TASKS_QUEUE"
        value = google_cloud_tasks_queue.provisioning.name
      }
      env {
        name  = "CLOUD_TASKS_REGION"
        value = var.region
      }
      env {
        name  = "CONTROLLER_SERVICE_ACCOUNT"
        value = google_service_account.controller_service_account.email
      }
      env {
        name = "AGENT_JWT_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.agent_jwt_secret.secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "DAPR_STATE_STORE_NAME"
        value = "statestore"
      }
      env {
        name  = "DAPR_GRPC_PORT"
        value = "50001"
      }

    }
    containers {
      name    = "daprd"
      image   = var.daprd_image
      command = ["/daprd"]
      args = [
        "--app-id", "controller",
        "--dapr-http-port", "3500",
        "--dapr-grpc-port", "50001",
        "--resources-path", "/dapr/components",
        "--log-level", "info"
      ]

      startup_probe {
        http_get {
          path = "/v1.0/healthz/outbound"
          port = 3500
        }
        initial_delay_seconds = 5
        period_seconds        = 5
        timeout_seconds       = 3
        failure_threshold     = 12
      }
      resources {
        limits = {
          memory = "256Mi"
        }
      }
      volume_mounts {
        name       = "dapr-secrets"
        mount_path = "/dapr/secrets"
      }
    }
    volumes {
      name = "dapr-secrets"
      secret {
        secret       = google_secret_manager_secret.postgres_connection_string.id
        default_mode = "0444"
        items {
          path    = "secrets.json"
          version = "latest"
        }
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

resource "random_password" "agent_jwt_secret" {
  length  = 32
  special = false
}

resource "google_secret_manager_secret" "agent_jwt_secret" {
  secret_id = "${var.environment}-agent_jwt_secret"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "agent_jwt_secret_value" {
  secret                 = google_secret_manager_secret.agent_jwt_secret.id
  secret_data_wo_version = 0
  secret_data_wo         = random_password.agent_jwt_secret.result
}

resource "google_secret_manager_secret_iam_member" "secret-access-agent_jwt_secret" {
  secret_id  = google_secret_manager_secret.agent_jwt_secret.id
  role       = "roles/secretmanager.secretAccessor"
  member     = "serviceAccount:${google_service_account.controller_service_account.email}"
  depends_on = [google_secret_manager_secret.agent_jwt_secret]
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
  secret_data_wo         = "dummy"
}

resource "google_secret_manager_secret_iam_member" "secret-access-firebase_api_key" {
  secret_id  = google_secret_manager_secret.firebase_api_key.id
  role       = "roles/secretmanager.secretAccessor"
  member     = "serviceAccount:${google_service_account.controller_service_account.email}"
  depends_on = [google_secret_manager_secret.firebase_api_key]
}

resource "google_secret_manager_secret" "postgres_connection_string" {
  secret_id = "${var.environment}-postgres-connection-string"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "postgres_connection_string_dummy" {
  secret                 = google_secret_manager_secret.postgres_connection_string.id
  secret_data_wo_version = 0
  secret_data_wo         = jsonencode({ "postgres-connection-string" = "postgres://REPLACE-ME:REPLACE-ME@REPLACE-ME:5432/metio?sslmode=require" })
}

resource "google_secret_manager_secret_iam_member" "secret-access-postgres_connection_string" {
  secret_id  = google_secret_manager_secret.postgres_connection_string.id
  role       = "roles/secretmanager.secretAccessor"
  member     = "serviceAccount:${google_service_account.controller_service_account.email}"
  depends_on = [google_secret_manager_secret.postgres_connection_string]
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
