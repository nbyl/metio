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

resource "google_compute_address" "static" {
  name = "ipv4-address"
}

resource "google_service_account" "minecraft-server-sa" {
  account_id   = "minecraft-server"
  display_name = "Custom SA for VM Instance"
}

resource "google_compute_disk" "primary" {
  name  = "minecraft-data"
  type  = "pd-ssd"

  physical_block_size_bytes = 4096
}

resource "google_compute_instance" "minecraft-server" {
  name         = "minecraft-server"
  machine_type = "e2-micro"

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
    }
  }

  attached_disk {
    source = google_compute_disk.primary.id
    mode   = "READ_WRITE"
  }

  network_interface {
    network = "default"
    access_config {
      nat_ip = google_compute_address.static.address
    }
  }

  tags = ["minecraft-server"]

  service_account {
    # Google recommends custom service accounts that have cloud-platform scope and permissions granted via IAM Roles.
    email  = google_service_account.minecraft-server-sa.email
    scopes = ["cloud-platform"]
  }
}