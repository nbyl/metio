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

variable "machine_type" {
  description = "The machine type to be used for the server VM."
  type        = string
  default     = "e2-micro"
}

variable "desired_status" {
  description = "The desired status of the server."
  type        = string
  default     = "TERMINATED"
}

variable "minecraft_version" {
  description = "The version of the Minecraft server to deploy."
  type        = string
  default     = "1.21.5"
}

variable "controller_image" {
  description = "The container image for the controller service."
  type        = string
  default     = "us-docker.pkg.dev/cloudrun/container/hello"
}

variable "environment" {
  description = "The deployment environment (e.g., dev, staging, prod)."
  type        = string
  default     = "development"
}
