# Cloud SQL provisioning (postgres_mode = "cloudsql")

locals {
  postgres_db_name = "metio"
}

resource "google_project_service" "sqladmin" {
  count                      = var.postgres_mode == "cloudsql" ? 1 : 0
  project                    = var.project_id
  service                    = "sqladmin.googleapis.com"
  disable_on_destroy         = false
  disable_dependent_services = false
}

resource "random_password" "postgres_password" {
  count   = var.postgres_mode == "cloudsql" ? 1 : 0
  length  = 16
  special = false
}

resource "google_sql_database_instance" "postgres" {
  count            = var.postgres_mode == "cloudsql" ? 1 : 0
  name             = "${var.environment}-metio-db"
  database_version = "POSTGRES_18"
  region           = var.region

  settings {
    tier = "db-f1-micro"

    ip_configuration {
      ipv4_enabled = true
      ssl_mode     = "ENCRYPTED_ONLY"
    }
  }

  deletion_protection = false

  depends_on = [google_project_service.sqladmin]
}

resource "google_sql_database" "postgres" {
  count    = var.postgres_mode == "cloudsql" ? 1 : 0
  name     = local.postgres_db_name
  instance = google_sql_database_instance.postgres[0].name
}

resource "google_sql_user" "postgres" {
  count              = var.postgres_mode == "cloudsql" ? 1 : 0
  name               = local.postgres_db_name
  instance           = google_sql_database_instance.postgres[0].name
  password_wo        = random_password.postgres_password[0].result
  password_wo_version = 1
}

resource "google_secret_manager_secret_version" "postgres_connection_string_cloudsql" {
  count                = var.postgres_mode == "cloudsql" ? 1 : 0
  secret               = google_secret_manager_secret.postgres_connection_string.id
  secret_data_wo_version = 0
  secret_data_wo       = jsonencode({
    "postgres-connection-string" = "postgres://${google_sql_user.postgres[0].name}:${random_password.postgres_password[0].result}@${google_sql_database_instance.postgres[0].public_ip_address}:5432/${google_sql_database.postgres[0].name}?sslmode=require"
  })
}
