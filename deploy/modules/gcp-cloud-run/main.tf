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

resource "google_firestore_database" "dapr_statestore" {
  project     = var.project_id
  name        = "(default)"
  location_id = var.region
  type        = "DATASTORE_MODE"
}
