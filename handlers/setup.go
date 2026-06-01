package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"gitlab.com/nbyl/metio/services"
)

type ValidationServiceInterface interface {
	Validate(ctx context.Context) (*services.ValidationResult, error)
}

func validateSetupHandler(w http.ResponseWriter, r *http.Request) {
	result, err := validationService.Validate(r.Context())
	if err != nil {
		log.Printf("validation handler error: %v", err)
		WriteJSONError(w, "validation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}
