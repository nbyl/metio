package setup

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type StatusResponse struct {
	Initialized bool        `json:"initialized"`
	ServerCount int         `json:"serverCount"`
	Checks      interface{} `json:"checks"`
}

func ValidateSetupHandler(w http.ResponseWriter, r *http.Request) {
	vCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := ValidationService.Validate(vCtx)
	if err != nil {
		log.Printf("validation handler error: %v", err)
		writeJSONError(w, "validation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	initialized := SetupService.IsInitialized(r.Context())

	serverCount, err := SetupService.ServerCount(r.Context())
	if err != nil {
		log.Printf("status handler: failed to count servers: %v", err)
		writeJSONError(w, "failed to check server count", http.StatusInternalServerError)
		return
	}

	vCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	checks, err := ValidationService.Validate(vCtx)
	if err != nil {
		log.Printf("status handler: validation error (returning partial data): %v", err)
	}

	resp := StatusResponse{
		Initialized: initialized,
		ServerCount: serverCount,
		Checks:      checks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func InitializeHandler(w http.ResponseWriter, r *http.Request) {
	if err := SetupService.Initialize(r.Context()); err != nil {
		log.Printf("initialize handler error: %v", err)
		writeJSONError(w, "initialization failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
