resource "google_storage_bucket" "functions_source_bucket" {
  name                        = "${random_id.default.hex}-bylcraft-source"
  location                    = "EUROPE-WEST3"
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
  location    = "europe-west3"
  description = "starts the minecraft server"

  build_config {
    runtime     = "nodejs22"
    entry_point = "helloHttp" # Set the entry point
    source {
      storage_source {
        bucket     = data.google_storage_bucket_object.functions_source_object.bucket
        object     = data.google_storage_bucket_object.functions_source_object.name
        generation = data.google_storage_bucket_object.functions_source_object.generation
      }
    }
  }

  service_config {
    max_instance_count = 1
    available_memory   = "256M"
    timeout_seconds    = 60
  }
}

resource "google_cloud_run_service_iam_member" "member" {
  location = google_cloudfunctions2_function.start_function.location
  service  = google_cloudfunctions2_function.start_function.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "function_uri" {
  value = google_cloudfunctions2_function.start_function.service_config[0].uri
}
