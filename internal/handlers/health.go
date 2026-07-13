package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

var daprHealthURL = "http://localhost:3500/v1.0/healthz/outbound"

var daprHealthCheck = func() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(daprHealthURL)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusNoContent
}

var requireDaprCheck = func() bool {
	return os.Getenv("DB_BACKEND") == "dapr"
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")

	if requireDaprCheck() && !daprHealthCheck() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}
