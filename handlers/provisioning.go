package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/pulumi"
	"gitlab.com/nbyl/metio/pulumi/programs"
	"gitlab.com/nbyl/metio/services"
)

type ServerConfigRequest struct {
	Region           string `json:"region"`
	Zone             string `json:"zone"`
	MachineType      string `json:"machineType"`
	MinecraftVersion string `json:"minecraftVersion"`
	DiskSizeGB       int    `json:"diskSizeGB"`
	BackupBucket     string `json:"backupBucket"`
	RCONPassword     string `json:"rconPassword"`
}

type CreateServerResponse struct {
	Success     bool   `json:"success"`
	ServerID    string `json:"serverId,omitempty"`
	Error       string `json:"error,omitempty"`
	OperationID string `json:"operationId,omitempty"`
}

type ProvisioningStatusResponse struct {
	ID          string            `json:"id"`
	Operation   string            `json:"operation"`
	State       string            `json:"state"`
	CurrentStep string            `json:"currentStep"`
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

func createServerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req ServerConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cfg := config.Load()

	provisioningService, err := getProvisioningService(ctx, cfg)
	if err != nil {
		log.Printf("Error creating provisioning service: %v", err)
		writeJSONError(w, "failed to create provisioning service", http.StatusInternalServerError)
		return
	}

	serverID := fmt.Sprintf("%s-server", cfg.Environment)

	programConfig := &programs.ServerConfig{
		Region:            req.Region,
		Zone:              req.Zone,
		MachineType:       req.MachineType,
		MinecraftVersion:  req.MinecraftVersion,
		DiskSizeGB:        req.DiskSizeGB,
		Environment:       cfg.Environment,
		BackupBucket:      req.BackupBucket,
		MachineAgentImage: viper.GetString("MACHINE_AGENT_IMAGE"),
		GCPProject:        cfg.ProjectID,
		InstanceName:      serverID,
		RCONPassword:      req.RCONPassword,
	}

	err = provisioningService.CreateServer(ctx, serverID, programConfig)
	if err != nil {
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		log.Printf("Error creating server: %v", err)
		writeJSONError(w, fmt.Sprintf("failed to create server: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateServerResponse{
		Success:  true,
		ServerID: serverID,
	})
}

func updateServerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := r.URL.Query()
	serverID := vars.Get("id")
	if serverID == "" {
		serverID = fmt.Sprintf("%s-server", config.Load().Environment)
	}

	var req ServerConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cfg := config.Load()

	provisioningService, err := getProvisioningService(ctx, cfg)
	if err != nil {
		log.Printf("Error creating provisioning service: %v", err)
		writeJSONError(w, "failed to create provisioning service", http.StatusInternalServerError)
		return
	}

	programConfig := &programs.ServerConfig{
		Region:            req.Region,
		Zone:              req.Zone,
		MachineType:       req.MachineType,
		MinecraftVersion:  req.MinecraftVersion,
		DiskSizeGB:        req.DiskSizeGB,
		Environment:       cfg.Environment,
		BackupBucket:      req.BackupBucket,
		MachineAgentImage: viper.GetString("MACHINE_AGENT_IMAGE"),
		GCPProject:        cfg.ProjectID,
		InstanceName:      serverID,
		RCONPassword:      req.RCONPassword,
	}

	err = provisioningService.UpdateServer(ctx, serverID, programConfig)
	if err != nil {
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		log.Printf("Error updating server: %v", err)
		writeJSONError(w, fmt.Sprintf("failed to update server: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateServerResponse{
		Success:  true,
		ServerID: serverID,
	})
}

func deleteServerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := r.URL.Query()
	serverID := vars.Get("id")
	if serverID == "" {
		serverID = fmt.Sprintf("%s-server", config.Load().Environment)
	}

	cfg := config.Load()

	provisioningService, err := getProvisioningService(ctx, cfg)
	if err != nil {
		log.Printf("Error creating provisioning service: %v", err)
		writeJSONError(w, "failed to create provisioning service", http.StatusInternalServerError)
		return
	}

	err = provisioningService.DestroyServer(ctx, serverID)
	if err != nil {
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		log.Printf("Error deleting server: %v", err)
		writeJSONError(w, fmt.Sprintf("failed to delete server: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateServerResponse{
		Success:  true,
		ServerID: serverID,
	})
}

func getOperationStatusHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := r.URL.Query()
	serverID := vars.Get("id")
	if serverID == "" {
		serverID = fmt.Sprintf("%s-server", config.Load().Environment)
	}

	cfg := config.Load()

	provisioningService, err := getProvisioningService(ctx, cfg)
	if err != nil {
		log.Printf("Error creating provisioning service: %v", err)
		writeJSONError(w, "failed to create provisioning service", http.StatusInternalServerError)
		return
	}

	status, err := provisioningService.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		log.Printf("Error getting provisioning status: %v", err)
		writeJSONError(w, "provisioning status not found", http.StatusNotFound)
		return
	}

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ProvisioningStatusResponse{
		ID:          status.ID,
		Operation:   status.Operation.String(),
		State:       status.State.String(),
		CurrentStep: status.CurrentStep,
		Steps:       steps,
		Error:       status.Error,
		StartedAt:   status.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		CompletedAt: completedAt,
		Outputs:     status.Outputs,
	})
}

func getProvisioningService(ctx context.Context, cfg config.Config) (*services.ProvisioningService, error) {
	stateBucket := viper.GetString("PULUMI_STATE_BUCKET")
	if stateBucket == "" {
		return nil, fmt.Errorf("PULUMI_STATE_BUCKET not configured")
	}

	workspaceManager, err := pulumi.NewWorkspaceManager(ctx, cfg.ProjectID, stateBucket)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace manager: %w", err)
	}

	dbConn, err := cfg.NewDBConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create db connection: %w", err)
	}

	return services.NewProvisioningService(workspaceManager, dbConn), nil
}
