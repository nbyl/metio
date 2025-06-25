resource "google_storage_bucket" "functions_source_bucket" {
  name                        = "${random_id.default.hex}-bylcraft-source"
  location                    = var.region
  uniform_bucket_level_access = true
}

data "archive_file" "source_file" {
  type        = "zip"
  output_path = "/tmp/functions-source.zip"
  source_dir  = "functions/"
  excludes    = ["lib", "node_modules"]
}

resource "google_storage_bucket_object" "functions_source_object" {
  name   = "functions-source.zip"
  bucket = google_storage_bucket.functions_source_bucket.name
  source = data.archive_file.source_file.output_path
}

data "google_storage_bucket_object" "functions_source_object" {
  name   = google_storage_bucket_object.functions_source_object.name
  bucket = google_storage_bucket_object.functions_source_object.bucket
}

resource "google_cloudfunctions2_function" "start_function" {
  name        = "start-server"
  location    = var.region
  description = "starts the minecraft server"

  build_config {
    runtime     = "nodejs22"
    entry_point = "startServer" # Set the entry point
    source {
      storage_source {
        bucket     = data.google_storage_bucket_object.functions_source_object.bucket
        object     = data.google_storage_bucket_object.functions_source_object.name
        generation = data.google_storage_bucket_object.functions_source_object.generation
      }
    }
  }

  service_config {
    environment_variables = {
      GCP_PROJECT = var.project_id
      GCP_ZONE    = var.zone
    }
    max_instance_count = 1
    available_memory   = "512M"
    timeout_seconds    = 60
  }
}

data "google_iam_policy" "admins" {
  binding {
    role    = "roles/run.invoker"
    members = var.admin_members
  }
}

resource "google_cloud_run_service_iam_policy" "policy" {
  location    = google_cloudfunctions2_function.start_function.location
  service     = google_cloudfunctions2_function.start_function.name
  policy_data = data.google_iam_policy.admins.policy_data
}

output "start_url" {
  value = google_cloudfunctions2_function.start_function.service_config[0].uri
}
