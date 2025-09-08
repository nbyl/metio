resource "google_storage_bucket" "minecraft-backups" {
  name                     = "${terraform.workspace}-minecraft-backups-bucket"
  location                 = "europe-west3"
  force_destroy            = true
  public_access_prevention = "enforced"
}
