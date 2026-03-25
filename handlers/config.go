package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/spf13/viper"
)

// AppConfig represents the configuration exposed to the frontend
type AppConfig struct {
	GCPProject   string `json:"gcpProject"`
	InstanceName string `json:"instanceName"`
}

// configHandler returns application configuration for the frontend
// This endpoint does not require authentication as the config is not sensitive
func configHandler(w http.ResponseWriter, r *http.Request) {
	gcpProject := viper.GetString("GCP_PROJECT")

	config := AppConfig{
		GCPProject:   gcpProject,
		InstanceName: viper.GetString("INSTANCE_NAME"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}
