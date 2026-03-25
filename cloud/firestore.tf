# Firestore Security Rules
# Allow authenticated users to read server status

resource "google_firebaserules_ruleset" "firestore" {
  project = var.project_id

  source {
    files {
      content = <<EOF
rules_version = '2';
service cloud.firestore {
  match /databases/{database}/documents {
    // Allow authenticated users to read server status
    match /instances/{instanceName}/data/{document=**} {
      allow read: if request.auth != null;
    }
  }
}
EOF
      name    = "firestore.rules"
    }
  }
}

# Rules for the named database (to be used once Firebase SDK issue is resolved)
resource "google_firebaserules_release" "firestore" {
  project      = var.project_id
  name         = "cloud.firestore/databases/${google_firestore_database.metio_firestore.name}/rules"
  ruleset_name = google_firebaserules_ruleset.firestore.name
}

# Rules for the default database (temporary workaround for Firebase SDK issue)
resource "google_firebaserules_release" "firestore_default" {
  project      = var.project_id
  name         = "cloud.firestore/databases/(default)/rules"
  ruleset_name = google_firebaserules_ruleset.firestore.name
}
