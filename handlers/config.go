package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/spf13/viper"
)

// AppConfig represents the configuration exposed to the frontend
type AppConfig struct {
	GCPProject         string `json:"gcpProject"`
	FirestoreDatabase  string `json:"firestoreDatabase"`
	InstanceName       string `json:"instanceName"`
	FirebaseApiKey     string `json:"firebaseApiKey"`
	FirebaseAuthDomain string `json:"firebaseAuthDomain"`
}

// configHandler returns application configuration for the frontend
// This endpoint does not require authentication as the config is not sensitive
func configHandler(w http.ResponseWriter, r *http.Request) {
	environment := viper.GetString("ENVIRONMENT")
	region := viper.GetString("REGION")
	gcpProject := viper.GetString("GCP_PROJECT")

	config := AppConfig{
		GCPProject:         gcpProject,
		FirestoreDatabase:  environment + "-" + region + "-metio-db",
		InstanceName:       viper.GetString("INSTANCE_NAME"),
		FirebaseApiKey:     viper.GetString("FIREBASE_API_KEY"),
		FirebaseAuthDomain: gcpProject + ".firebaseapp.com",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}
