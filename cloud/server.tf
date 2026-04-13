locals {
  user_data = templatefile("${path.module}/../templates/server_cloud_config.tftpl", {
    region            = var.region
    gcpProject        = var.project_id
    environment       = var.environment
    instanceName      = "${var.environment}-minecraft-server"
    backupBucket      = google_storage_bucket.minecraft-backups.name
    machineAgentImage = var.machine_agent_image
    minecraftVersion  = var.minecraft_version
    rconPassword      = var.rcon_password
  })
}

resource "terraform_data" "user-data" {
  input = local.user_data
}

resource "terraform_data" "machine_agent_image" {
  input = var.machine_agent_image
}

resource "google_service_account" "minecraft-server-sa" {
  account_id   = "${var.environment}-ms"
  display_name = "Custom SA for VM Instance"
}

resource "google_project_iam_member" "sa_storage_object_user" {
  project = var.project_id
  role    = "roles/storage.objectUser"
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

resource "google_project_iam_member" "sa_logging_metric_writer" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_project_iam_member" "sa_trace_writer" {
  project = var.project_id
  role    = "roles/cloudtrace.agent"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_project_iam_member" "sa_container_registry_reader" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_project_iam_member" "sa_firestore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_project_iam_member" "sa_serviceusage_consumer" {
  project = var.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_project_iam_member" "sa_compute_instance_admin" {
  project = var.project_id
  role    = "roles/compute.instanceAdmin.v1"
  member  = "serviceAccount:${google_service_account.minecraft-server-sa.email}"
}

resource "google_compute_disk" "primary" {
  name = "${var.environment}-minecraft-data"
  type = "pd-standard"
  size = 10

  physical_block_size_bytes = 4096
}

resource "google_compute_address" "static" {
  name = "${var.environment}-minecraft-server"
}

resource "google_compute_firewall" "minecraft-server-firewall" {
  name    = "${var.environment}-minecraft-server"
  network = "default"

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["25565"]
  }

  source_ranges = ["0.0.0.0/0"]

  target_tags = ["${var.environment}-minecraft-server"]
}

resource "google_compute_instance" "minecraft-server" {
  name           = "${var.environment}-minecraft-server"
  machine_type   = var.machine_type
  desired_status = var.desired_status

  boot_disk {
    initialize_params {
      image = "cos-cloud/cos-stable"
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

  scheduling {
    preemptible                 = true
    automatic_restart           = false
    provisioning_model         = "SPOT"
    instance_termination_action = "STOP"
  }

  tags = ["${var.environment}-minecraft-server", var.environment]

  service_account {
    # Google recommends custom service accounts that have cloud-platform scope and permissions granted via IAM Roles.
    email  = google_service_account.minecraft-server-sa.email
    scopes = ["cloud-platform"]
  }

  metadata = {
    user-data = local.user_data
  }

  lifecycle {
    replace_triggered_by = [
      terraform_data.user-data,
      terraform_data.machine_agent_image
    ]
  }
}

resource "google_compute_resource_policy" "daily_shutdown" {
  name   = "${var.environment}-daily-shutdown"
  region = var.region

  instance_schedule_policy {
    vm_stop_schedule {
      schedule = "0 21 * * *"
    }

    time_zone = "Europe/Berlin"
  }
}

resource "google_compute_resource_policy_attachment" "daily_shutdown_attachment" {
  name     = google_compute_resource_policy.daily_shutdown.name
  instance = google_compute_instance.minecraft-server.name
  zone     = var.zone
}
