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

variable "admin_members" {
  description = "List of members to be granted admin access."
  type        = list(string)
  default     = []
}
