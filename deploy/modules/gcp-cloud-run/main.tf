terraform {
  required_providers {
    google = {
      source = "opentffoundation/google"
    }
  }
}

resource "random_id" "default" {
  byte_length = 8
}

data "google_project" "current" {
  project_id = var.project_id
}
