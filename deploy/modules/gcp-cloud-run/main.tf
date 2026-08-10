terraform {
  required_version = ">= 1.6.0"

  required_providers {
    google = {
      source  = "opentffoundation/google"
      version = "~> 6.39"
    }
  }
}

resource "random_id" "default" {
  byte_length = 8
}

data "google_project" "current" {
  project_id = var.project_id
}
