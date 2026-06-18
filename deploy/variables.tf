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
  description = "The container image for the controller service."
  type        = string
  default     = "us-docker.pkg.dev/cloudrun/container/hello"
}

variable "machine_agent_image" {
  description = "Docker image for metio-machine-agent"
  type        = string
  default     = "us-central1-docker.pkg.dev/cloudrun/container/hello"
}

variable "environment" {
  description = "The deployment environment (e.g., dev, staging, prod)."
  type        = string
  default     = "development"
}


