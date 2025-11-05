# Pub/Sub infrastructure for instance lifecycle events

resource "google_pubsub_topic" "instance_lifecycle" {
  name = "${var.environment}-instance-lifecycle"
}

resource "google_pubsub_subscription" "instance_lifecycle_push" {
  name  = "${var.environment}-instance-lifecycle-push"
  topic = google_pubsub_topic.instance_lifecycle.name

  push_config {
    push_endpoint = "${google_cloud_run_v2_service.controller.uri}/events"
    oidc_token {
      service_account_email = google_service_account.controller_service_account.email
      audience              = google_cloud_run_v2_service.controller.uri
    }
  }

  # Retry configuration for failed deliveries
  ack_deadline_seconds = 60
  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }

  # Dead letter topic for messages that can't be delivered
  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.instance_lifecycle_dead_letter.id
    max_delivery_attempts = 5
  }
}

# Dead letter topic for failed messages
resource "google_pubsub_topic" "instance_lifecycle_dead_letter" {
  name = "${var.environment}-instance-lifecycle-dead-letter"
}

# Log sink to route compute audit logs to Pub/Sub
resource "google_logging_project_sink" "compute_audit_logs" {
  name        = "${var.environment}-compute-audit-logs"
  destination = "pubsub.googleapis.com/projects/${var.project_id}/topics/${google_pubsub_topic.instance_lifecycle.name}"

  # Filter for compute instance lifecycle events
  filter = "protoPayload.methodName=\"v1.compute.instances.stop\" OR protoPayload.methodName=\"v1.compute.instances.start\" OR protoPayload.methodName=\"v1.compute.instances.preempted\""

  custom_writer_identity = "serviceAccount:${google_service_account.logging_service_account.email}"
}

# Grant the sink's service account permission to publish to the topic
resource "google_pubsub_topic_iam_member" "log_sink_publisher" {
  topic  = google_pubsub_topic.instance_lifecycle.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${google_logging_project_sink.compute_audit_logs.writer_identity}"
}