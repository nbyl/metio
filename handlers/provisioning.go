package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"gitlab.com/nbyl/metio/db"
)

type ProvisioningStatusResponse struct {
	ID          string            `json:"id"`
	Operation   string            `json:"operation"`
	State       string            `json:"state"`
	CurrentStep string            `json:"currentStep"`
	Progress    int               `json:"progress"`
	Steps       []StepResponse    `json:"steps"`
	Error       string            `json:"error,omitempty"`
	StartedAt   string            `json:"startedAt"`
	CompletedAt *string           `json:"completedAt,omitempty"`
	Outputs     map[string]string `json:"outputs,omitempty"`
}

type StepResponse struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp,omitempty"`
}

func getServerProvisioningStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	serverID := vars["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}

	if provisioningService == nil {
		writeJSONError(w, "provisioning service not available", http.StatusServiceUnavailable)
		return
	}

	status, err := provisioningService.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		log.Printf("Error getting provisioning status for %s: %v", serverID, err)
		writeJSONError(w, "provisioning status not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toProvisioningStatusResponse(status))
}

func toProvisioningStatusResponse(status *db.ProvisioningStatus) ProvisioningStatusResponse {
	steps := make([]StepResponse, len(status.Steps))
	for i, step := range status.Steps {
		steps[i] = StepResponse{
			Name:      step.Name,
			Status:    step.Status.String(),
			Message:   step.Message,
			Timestamp: step.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	var completedAt *string
	if status.CompletedAt != nil {
		formatted := status.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		completedAt = &formatted
	}

	return ProvisioningStatusResponse{
		ID:          status.ID,
		Operation:   status.Operation.String(),
		State:       status.State.String(),
		CurrentStep: status.CurrentStep,
		Progress:    calculateProgress(status),
		Steps:       steps,
		Error:       status.Error,
		StartedAt:   status.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		CompletedAt: completedAt,
		Outputs:     status.Outputs,
	}
}

func calculateProgress(status *db.ProvisioningStatus) int {
	if len(status.Steps) == 0 {
		return 0
	}

	if status.State == db.ProvisioningStateCompleted {
		return 100
	}

	completed := 0
	for _, step := range status.Steps {
		if step.Status == db.ProvisioningStateCompleted {
			completed++
		}
	}

	return (completed * 100) / len(status.Steps)
}
