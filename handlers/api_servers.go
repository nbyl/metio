package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/pulumi/programs"
)

type ShutdownScheduleInput struct {
	Enabled  bool   `json:"enabled"`
	Time     string `json:"time,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type CreateServerRequest struct {
	Name             string                 `json:"name"`
	Region           string                 `json:"region"`
	Zone             string                 `json:"zone"`
	MachineType      string                 `json:"machineType"`
	MinecraftVersion string                 `json:"minecraftVersion"`
	DiskSizeGB       int                    `json:"diskSizeGB,omitempty"`
	ShutdownSchedule *ShutdownScheduleInput `json:"shutdownSchedule,omitempty"`
}

type UpdateServerRequest struct {
	Name             *string                `json:"name,omitempty"`
	Region           *string                `json:"region,omitempty"`
	Zone             *string                `json:"zone,omitempty"`
	MachineType      *string                `json:"machineType,omitempty"`
	MinecraftVersion *string                `json:"minecraftVersion,omitempty"`
	DiskSizeGB       *int                   `json:"diskSizeGB,omitempty"`
	ShutdownSchedule *ShutdownScheduleInput `json:"shutdownSchedule,omitempty"`
}

type ServerConfigJSON struct {
	Name                        string                 `json:"name"`
	Region                      string                 `json:"region"`
	Zone                        string                 `json:"zone"`
	MachineType                 string                 `json:"machineType"`
	MinecraftVersion            string                 `json:"minecraftVersion"`
	DiskSizeGB                  int                    `json:"diskSizeGB"`
	InfraVersion                int                    `json:"infraVersion,omitempty"`
	DeployedByControllerVersion string                 `json:"deployedByControllerVersion,omitempty"`
	ShutdownSchedule            *ShutdownScheduleInput `json:"shutdownSchedule,omitempty"`
	CreatedAt                   string                 `json:"createdAt"`
	UpdatedAt                   string                 `json:"updatedAt"`
}

type StatusResponse struct {
	Players           PlayersJSON `json:"players"`
	Timestamp         string      `json:"timestamp"`
	Uptime            string      `json:"uptime"`
	ServerState       string      `json:"serverState"`
	InstanceIP        string      `json:"instanceIP"`
	Version           string      `json:"version"`
	WhitelistEnabled  bool        `json:"whitelistEnabled"`
	ScheduledShutdown *string     `json:"scheduledShutdown,omitempty"`
}

type PlayersJSON struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

type ServerResponse struct {
	ID                  string           `json:"id"`
	Config              ServerConfigJSON `json:"config"`
	Status              *StatusResponse  `json:"status,omitempty"`
	CurrentInfraVersion int              `json:"currentInfraVersion"`
	Outdated            bool             `json:"outdated"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func shutdownScheduleToInput(s *db.ShutdownSchedule) *ShutdownScheduleInput {
	if s == nil {
		return nil
	}
	return &ShutdownScheduleInput{
		Enabled:  s.Enabled,
		Time:     s.Time,
		Timezone: s.Timezone,
	}
}

func shutdownScheduleFromInput(s *ShutdownScheduleInput) *db.ShutdownSchedule {
	if s == nil {
		return nil
	}
	return &db.ShutdownSchedule{
		Enabled:  s.Enabled,
		Time:     s.Time,
		Timezone: s.Timezone,
	}
}

func serverConfigToJSON(cfg *db.ServerConfig) ServerConfigJSON {
	return ServerConfigJSON{
		Name:                        cfg.Name,
		Region:                      cfg.Region,
		Zone:                        cfg.Zone,
		MachineType:                 cfg.MachineType,
		MinecraftVersion:            cfg.MinecraftVersion,
		DiskSizeGB:                  cfg.DiskSizeGB,
		InfraVersion:                cfg.InfraVersion,
		DeployedByControllerVersion: cfg.DeployedByControllerVersion,
		ShutdownSchedule:            shutdownScheduleToInput(cfg.ShutdownSchedule),
		CreatedAt:                   cfg.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                   cfg.UpdatedAt.Format(time.RFC3339),
	}
}

func createServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	shutdownSchedule := shutdownScheduleFromInput(req.ShutdownSchedule)
	serverConfig := &db.ServerConfig{
		Name:             req.Name,
		Region:           req.Region,
		Zone:             req.Zone,
		MachineType:      req.MachineType,
		MinecraftVersion: req.MinecraftVersion,
		DiskSizeGB:       req.DiskSizeGB,
		ShutdownSchedule: shutdownSchedule,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := db.ValidateServerConfig(serverConfig); err != nil {
		writeJSONError(w, fmt.Sprintf("validation error: %s", err.Error()), http.StatusBadRequest)
		return
	}

	dbConn, cfg, err := getDBConnection(ctx)
	if err != nil {
		log.Printf("Error creating db connection: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverID := uuid.New().String()
	serverConfig.ID = serverID

	if err := dbConn.CreateServerConfig(ctx, serverID, serverConfig); err != nil {
		log.Printf("Error creating server config: %v", err)
		writeJSONError(w, "failed to create server config", http.StatusInternalServerError)
		return
	}

	if provisioningService == nil {
		writeJSONError(w, "provisioning service not available", http.StatusServiceUnavailable)
		return
	}

	programConfig := &programs.ServerConfig{
		Name:              req.Name,
		ServerID:          serverID,
		Region:            req.Region,
		Zone:              req.Zone,
		MachineType:       req.MachineType,
		MinecraftVersion:  req.MinecraftVersion,
		DiskSizeGB:        req.DiskSizeGB,
		Environment:       cfg.Environment,
		MachineAgentImage: viper.GetString("MACHINE_AGENT_IMAGE"),
		GCPProject:        cfg.ProjectID,
	}

	if err := provisioningService.CreateServer(ctx, serverID, programConfig); err != nil {
		log.Printf("Error starting server provisioning: %v", err)
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		writeJSONError(w, fmt.Sprintf("failed to start provisioning: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/api/servers/%s/provisioning", serverID))
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(ServerResponse{
		ID:                  serverID,
		Config:              serverConfigToJSON(serverConfig),
		CurrentInfraVersion: programs.CurrentInfraVersion,
		Outdated:            true, // newly created server is outdated until provisioning completes
	})
}

func listServers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		log.Printf("Error creating db connection: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	configs, err := dbConn.ListServerConfigs(ctx)
	if err != nil {
		log.Printf("Error listing server configs: %v", err)
		writeJSONError(w, "failed to list servers", http.StatusInternalServerError)
		return
	}

	responses := make([]ServerResponse, 0, len(configs))
	for _, cfg := range configs {
		outdated := cfg.InfraVersion < programs.CurrentInfraVersion
		responses = append(responses, ServerResponse{
			ID:                  cfg.ID,
			Config:              serverConfigToJSON(cfg),
			CurrentInfraVersion: programs.CurrentInfraVersion,
			Outdated:            outdated,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func getServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	serverID := vars["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		log.Printf("Error creating db connection: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	outdated := false
	if serverConfig.InfraVersion < programs.CurrentInfraVersion {
		outdated = true
	}

	response := ServerResponse{
		ID:                  serverID,
		Config:              serverConfigToJSON(serverConfig),
		CurrentInfraVersion: programs.CurrentInfraVersion,
		Outdated:            outdated,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func updateServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	serverID := vars["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}

	var req UpdateServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	dbConn, cfg, err := getDBConnection(ctx)
	if err != nil {
		log.Printf("Error creating db connection: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	existingConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	// Reject immutable field changes: region and zone.
	if req.Region != nil {
		writeJSONError(w, "region is immutable; create a new server to change location", http.StatusBadRequest)
		return
	}
	if req.Zone != nil {
		writeJSONError(w, "zone is immutable; create a new server to change location", http.StatusBadRequest)
		return
	}

	// Reject disk size decreases.
	if req.DiskSizeGB != nil && *req.DiskSizeGB < existingConfig.DiskSizeGB {
		writeJSONError(w, "disk size can only be increased", http.StatusBadRequest)
		return
	}

	// Save a snapshot of the original config before any mutation, for rollback.
	originalConfig := *existingConfig
	if err := dbConn.SaveConfigSnapshot(ctx, serverID, &originalConfig); err != nil {
		log.Printf("Error saving config snapshot: %v", err)
		writeJSONError(w, "failed to save config snapshot", http.StatusInternalServerError)
		return
	}

	if req.Name != nil {
		existingConfig.Name = *req.Name
	}
	if req.MachineType != nil {
		existingConfig.MachineType = *req.MachineType
	}
	if req.MinecraftVersion != nil {
		existingConfig.MinecraftVersion = *req.MinecraftVersion
	}
	if req.DiskSizeGB != nil {
		existingConfig.DiskSizeGB = *req.DiskSizeGB
	}
	if req.ShutdownSchedule != nil {
		existingConfig.ShutdownSchedule = shutdownScheduleFromInput(req.ShutdownSchedule)
	}
	existingConfig.UpdatedAt = time.Now()

	if err := db.ValidateServerConfig(existingConfig); err != nil {
		writeJSONError(w, fmt.Sprintf("validation error: %s", err.Error()), http.StatusBadRequest)
		return
	}

	// Classify the update type based on which fields changed.
	updateType := classifyUpdate(req, existingConfig)

	// Validate server state before accepting the update.
	if updateType == int(UpdateTypeRecreate) || updateType == int(UpdateTypeResize) {
		status, err := dbConn.GetStatus(ctx, serverID)
		if err == nil && status.ServerState != "" && !status.ServerState.IsRunning() {
			writeJSONError(w, fmt.Sprintf("server must be running to update. Current state: %s", status.ServerState), http.StatusBadRequest)
			return
		}
	}

	if err := dbConn.UpdateServerConfig(ctx, serverID, existingConfig); err != nil {
		log.Printf("Error updating server config: %v", err)
		writeJSONError(w, "failed to update server config", http.StatusInternalServerError)
		return
	}

	if provisioningService == nil {
		writeJSONError(w, "provisioning service not available", http.StatusServiceUnavailable)
		return
	}

	programConfig := &programs.ServerConfig{
		Name:              existingConfig.Name,
		ServerID:          serverID,
		Region:            existingConfig.Region,
		Zone:              existingConfig.Zone,
		MachineType:       existingConfig.MachineType,
		MinecraftVersion:  existingConfig.MinecraftVersion,
		DiskSizeGB:        existingConfig.DiskSizeGB,
		Environment:       cfg.Environment,
		MachineAgentImage: viper.GetString("MACHINE_AGENT_IMAGE"),
		GCPProject:        cfg.ProjectID,
	}

	if err := provisioningService.UpdateServer(ctx, serverID, programConfig, updateType); err != nil {
		log.Printf("Error starting server update: %v", err)
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		// Revert config snapshot on provisioning failure.
		if revertErr := provisioningService.RevertServerConfig(ctx, serverID); revertErr != nil {
			log.Printf("Error reverting config after failed update: %v", revertErr)
		}
		// Provide user-friendly guidance based on update type.
		msg := "Server update failed. "
		switch updateType {
		case int(UpdateTypeRecreate):
			msg += "The Minecraft server could not be updated. Your original configuration has been restored. If the problem persists, try stopping the server and retrying."
		case int(UpdateTypeResize):
			msg += "The machine type could not be changed. Your original configuration has been restored. The server will be restarted automatically."
		default:
			msg += "Your original configuration has been restored. Please try again or contact support."
		}
		writeJSONError(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/api/servers/%s/provisioning", serverID))
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(ServerResponse{
		ID:                  serverID,
		Config:              serverConfigToJSON(existingConfig),
		CurrentInfraVersion: programs.CurrentInfraVersion,
		Outdated:            true, // outdated until provisioning completes
	})
}

// classifyUpdate determines the UpdateType based on which fields were changed in the request.
func classifyUpdate(req UpdateServerRequest, config *db.ServerConfig) int {
	if req.MinecraftVersion != nil {
		return int(UpdateTypeRecreate)
	}
	if req.MachineType != nil {
		return int(UpdateTypeResize)
	}
	return int(UpdateTypeInPlace)
}

func deleteServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	serverID := vars["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		log.Printf("Error creating db connection: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	if _, err := dbConn.GetServerConfig(ctx, serverID); err != nil {
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	if provisioningService == nil {
		writeJSONError(w, "provisioning service not available", http.StatusServiceUnavailable)
		return
	}

	if err := provisioningService.DestroyServer(ctx, serverID); err != nil {
		log.Printf("Error starting server destruction: %v", err)
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		writeJSONError(w, fmt.Sprintf("failed to start destruction: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/api/servers/%s/provisioning", serverID))
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("server %s deletion started", serverID),
	})
}
