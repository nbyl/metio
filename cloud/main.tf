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
