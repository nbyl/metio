package setup

import (
	"encoding/json"
	"log"
	"net/http"
)

func ValidateSetupHandler(w http.ResponseWriter, r *http.Request) {
	result, err := ValidationService.Validate(r.Context())
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

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
