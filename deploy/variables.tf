variable "project_id" {
  description = "The ID of the project in which the resources will be created."
  type        = string
}

variable "region" {
  description = "The region where the resources will be created."
  type        = string
  default     = "europe-west3"
}

variable "zone" {
  description = "The zone where the resources will be created."
  type        = string
  default     = "europe-west3-a"
}

variable "admin_users" {
  description = "List of members to be granted admin access."
  type        = string
  default     = ""
}

variable "controller_image" {
  description = "The container image for the controller service. Override to pin a specific version."
  type        = string
  default     = "europe-docker.pkg.dev/metio-distribution/metio/controller:1.6.1" # x-release-please-version
}

variable "machine_agent_image" {
  description = "Docker image for metio-machine-agent. Override to pin a specific version."
  type        = string
  default     = "europe-docker.pkg.dev/metio-distribution/metio/machine-agent:1.6.1" # x-release-please-version
}

variable "environment" {
  description = "The deployment environment (e.g., dev, staging, prod)."
  type        = string
  default     = "development"
}

variable "daprd_image" {
  description = "The Dapr sidecar image with baked statestore components. Override to pin a specific version."
  type        = string
  default     = "europe-docker.pkg.dev/metio-distribution/metio/daprd:1.6.1" # x-release-please-version
}

variable "postgres_mode" {
  description = "PostgreSQL state backend topology: 'cloudsql' auto-provisions a Cloud SQL instance, 'byo' expects a user-filled connection-string secret (e.g. Neon, CockroachDB)."
  type        = string
  default     = "cloudsql"

  validation {
    condition     = contains(["cloudsql", "byo"], var.postgres_mode)
    error_message = "postgres_mode must be one of 'cloudsql' or 'byo'."
  }
}

variable "postgres_connection_string_secret_id" {
  description = "Secret Manager secret ID (same project) holding the BYO Postgres connection string. Required when postgres_mode = \"byo\"; the secret and its versions are managed outside OpenTofu. Ignored when \"cloudsql\"."
  type        = string
  default     = ""

  validation {
    condition     = var.postgres_mode != "byo" || trimspace(var.postgres_connection_string_secret_id) != ""
    error_message = "postgres_connection_string_secret_id must be set when postgres_mode is \"byo\"."
  }
}
