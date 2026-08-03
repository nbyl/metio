terraform {
  required_providers {
    google = {
      source  = "opentffoundation/google"
      version = "6.39.0"
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

  project_id          = var.project_id
  region              = var.region
  zone                = var.zone
  environment         = var.environment
  admin_users         = var.admin_users
  controller_image    = var.controller_image
  machine_agent_image = var.machine_agent_image
  daprd_image         = var.daprd_image
}
