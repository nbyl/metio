terraform {
  required_providers {
    google = {
      source  = "opentffoundation/google"
      version = "7.22.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

module "gcp_cloud_run" {
  source = "./modules/gcp-cloud-run"

  project_id                           = var.project_id
  region                               = var.region
  zone                                 = var.zone
  environment                          = var.environment
  admin_users                          = var.admin_users
  controller_image                     = var.controller_image
  machine_agent_image                  = var.machine_agent_image
  backup_image                         = var.backup_image
  daprd_image                          = var.daprd_image
  postgres_mode                        = var.postgres_mode
  postgres_connection_string_secret_id = var.postgres_connection_string_secret_id
  backup_deleted_server_retention_days = var.backup_deleted_server_retention_days
  backup_cleanup_schedule              = var.backup_cleanup_schedule
}
