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

resource "google_firestore_database" "metio_firestore" {
  project     = var.project_id
  name        = "${var.environment}-${var.region}-metio-db"
  location_id = var.region
  type        = "FIRESTORE_NATIVE"
}

resource "google_firestore_database" "dapr_statestore" {
  project     = var.project_id
  name        = "(default)"
  location_id = var.region
  type        = "DATASTORE_MODE"
}
