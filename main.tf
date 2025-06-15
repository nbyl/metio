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

resource "google_storage_bucket" "minecraft-backups" {
  name          = "minecraft-backups-bucket"
  location      = "europe-west3"
  force_destroy = true

  public_access_prevention = "enforced"
  retention_policy {
    retention_period = 7776000 # 90 days in seconds
  }
}

resource "google_compute_address" "static" {
  name = "ipv4-address"
}

resource "google_service_account" "minecraft-server-sa" {
  account_id   = "minecraft-server"
  display_name = "Custom SA for VM Instance" 
}

resource "google_project_iam_member" "sa_storage_object_viewer" {
  project = "minecraftbyl"
  role    = "roles/storage.objectViewer"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_project_iam_member" "sa_storage_object_creator" {
  project = "minecraftbyl"
  role    = "roles/storage.objectCreator"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_project_iam_member" "sa_logging_log_writer" {
  project = "minecraftbyl"
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_compute_disk" "primary" {
  name  = "minecraft-data"
  type  = "pd-ssd"
  size = 10

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
    device_name = "minecraft-data"
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

  metadata_startup_script = file("scripts/startup.sh")
  metadata = {
    shutdown-script = file("scripts/shutdown.sh")
  }
}