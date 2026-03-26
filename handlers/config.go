package handlers

import (
	"encoding/json"
	"net/http"

	"gitlab.com/nbyl/metio/config"
)

// AppConfig represents the configuration exposed to the frontend
type AppConfig struct {
	GCPProject   string `json:"gcpProject"`
	InstanceName string `json:"instanceName"`
}

// configHandler returns application configuration for the frontend
// This endpoint does not require authentication as the config is not sensitive
func configHandler(w http.ResponseWriter, r *http.Request) {
	cfg := config.Load()

	appConfig := AppConfig{
		GCPProject:   cfg.ProjectID,
		InstanceName: cfg.InstanceName,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(appConfig)
}
