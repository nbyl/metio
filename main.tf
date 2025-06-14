terraform {
  required_providers {
    google = {
      source = "opentffoundation/google"
      version = "6.39.0"
    }
  }
  backend "gcs" {
    bucket  = "minecraft-byl-tofu-state"
    prefix  = "state"
  }
}

provider "google" {
  project = "minecraftbyl"
  region  = "europe-west3"
  zone    = "europe-west3-a"
}

resource "google_compute_instance" "minecraft-server" {
  name         = "minecraft-server"
  machine_type = "e2-micro"

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
    }
  }

  network_interface {
    network = "default"
    access_config {
    }
  }
}