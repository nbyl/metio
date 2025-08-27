# GCP Overview and Instructions
# https://cloud.google.com/iam/docs/configuring-workload-identity-federation#oidc

provider "google" {
  project = var.gcp_project_name
  region  = "us-east1"
  zone    = "us-east1-c"
}

resource "random_id" "random" {
  byte_length = 4
}

resource "google_iam_workload_identity_pool" "gitlab-pool" {
  provider                  = google-beta
  workload_identity_pool_id = "gitlab-pool-${random_id.random.hex}"
  project = var.gcp_project_name
}

resource "google_iam_workload_identity_pool_provider" "gitlab-provider-jwt" {
  provider                           = google-beta
  workload_identity_pool_id          = google_iam_workload_identity_pool.gitlab-pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "gitlab-jwt-${random_id.random.hex}"
  project = var.gcp_project_name
  attribute_condition                = "assertion.namespace_path.startsWith(\"${var.gitlab_namespace_path}\")"
  attribute_mapping = {
    "google.subject"           = "assertion.sub", # Required
    "attribute.aud"            = "assertion.aud",
    "attribute.project_path"   = "assertion.project_path",
    "attribute.project_id"     = "assertion.project_id",
    "attribute.namespace_id"   = "assertion.namespace_id",
    "attribute.namespace_path" = "assertion.namespace_path",
    "attribute.user_email"     = "assertion.user_email",
    "attribute.ref"            = "assertion.ref",
    "attribute.ref_type"       = "assertion.ref_type",
  }
  oidc {
    issuer_uri        = var.gitlab_url
    allowed_audiences = [var.gitlab_url]
  }
}

resource "google_service_account" "gitlab-runner" {
  account_id   = "gitlab-runner-service-account"
  display_name = "Service Account for GitLab Runner"
}

resource "google_service_account_iam_binding" "gitlab-runner-oidc" {
  provider           = google-beta
  service_account_id = google_service_account.gitlab-runner.name
  role               = "roles/iam.workloadIdentityUser"

  members = [
    "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.gitlab-pool.name}/attribute.project_id/${var.gitlab_project_id}"
  ]

}

output "GCP_WORKLOAD_IDENTITY_PROVIDER" {
  value = google_iam_workload_identity_pool_provider.gitlab-provider-jwt.name
}

output "GCP_SERVICE_ACCOUNT" {
  value = google_service_account.gitlab-runner.email
}

