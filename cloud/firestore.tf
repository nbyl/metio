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
    match /instances/{instanceName}/data/status {
      allow read: if request.auth != null;
    }
    
    // Deny all other access by default
    match /{document=**} {
      allow read, write: if false;
    }
  }
}
EOF
      name    = "firestore.rules"
    }
  }
}

resource "google_firebaserules_release" "firestore" {
  project      = var.project_id
  name         = "cloud.firestore/databases/${google_firestore_database.metio_firestore.name}/rules"
  ruleset_name = google_firebaserules_ruleset.firestore.name
}
