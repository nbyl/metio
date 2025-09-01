resource "google_service_account" "minecraft-server-sa" {
  account_id   = "${terraform.workspace}-minecraft-server"
  display_name = "Custom SA for VM Instance"
}

resource "google_project_iam_member" "sa_storage_object_viewer" {
  project = var.project_id
  role    = "roles/storage.objectViewer"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_project_iam_member" "sa_storage_object_creator" {
  project = var.project_id
  role    = "roles/storage.objectCreator"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_project_iam_member" "sa_logging_log_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_compute_disk" "primary" {
  name = "${terraform.workspace}-minecraft-data"
  type = "pd-ssd"
  size = 10

  physical_block_size_bytes = 4096
}


resource "google_compute_address" "static" {
  name = "${terraform.workspace}-minecraft-server"
}

resource "google_compute_firewall" "minecraft-server-firewall" {
  name    = "${terraform.workspace}-minecraft-server"
  network = "default"

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["25565"]
  }

  source_ranges = ["0.0.0.0/0"]

  target_tags = ["${terraform.workspace}-minecraft-server"]
}

resource "google_compute_instance" "minecraft-server" {
  name         = "${terraform.workspace}-minecraft-server"
  machine_type = var.machine_type

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
    }
  }

  attached_disk {
    source      = google_compute_disk.primary.id
    mode        = "READ_WRITE"
    device_name = "minecraft-data"
  }

  network_interface {
    network = "default"
    access_config {
      nat_ip = google_compute_address.static.address
    }
  }

  tags = ["${terraform.workspace}-minecraft-server", terraform.workspace]

  service_account {
    # Google recommends custom service accounts that have cloud-platform scope and permissions granted via IAM Roles.
    email  = google_service_account.minecraft-server-sa.email
    scopes = ["cloud-platform"]
  }

  metadata_startup_script = templatefile("scripts/startup.sh.tftpl", { backup_bucket = resource.google_storage_bucket.minecraft-backups.name, server_jar_url = var.server_jar_url })
  metadata = {
    shutdown-script = templatefile("scripts/shutdown.sh.tftpl", {
      backup_bucket  = resource.google_storage_bucket.minecraft-backups.name,
      server_jar_url = var.server_jar_url
      }
    )
  }
}
