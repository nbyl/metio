terraform {
  required_providers {
    google = {
      source  = "opentffoundation/google"
      version = "6.39.0"
    }
  }
  backend "gcs" {
    bucket = "minecraft-byl-tofu-state"
    prefix = "state"
  }
}

provider "google" {
  project = "minecraftbyl"
  region  = "europe-west3"
  zone    = "europe-west3-a"
}
