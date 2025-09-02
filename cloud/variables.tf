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

variable "server_jar_url" {
  description = "The URL where the server.jar is downloaded"
  type        = string
  default     = "https://piston-data.mojang.com/v1/objects/e6ec2f64e6080b9b5d9b471b291c33cc7f509733/server.jar"
}
