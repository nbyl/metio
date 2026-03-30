resource "google_storage_bucket" "pulumi-state" {
  name                     = "${var.environment}-metio-pulumi-state"
  location                 = var.region
  force_destroy            = true
  public_access_prevention = "enforced"

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      age = 30
    }
    action {
      type = "Delete"
    }
  }
}
