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
