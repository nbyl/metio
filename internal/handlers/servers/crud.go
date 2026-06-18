package servers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/pulumi/programs"
)

func CreateServer(w http.ResponseWriter, r *http.Request) {
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

	dbConn, cfg, err := GetDBConnection(ctx)
	if err != nil {
		log.Printf("Error creating db connection: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverID := uuid.New().String()
	serverConfig.ID = serverID
	serverConfig.MachineAgentImage = cfg.MachineAgentImage

	if err := dbConn.CreateServerConfig(ctx, serverID, serverConfig); err != nil {
		log.Printf("Error creating server config: %v", err)
		writeJSONError(w, "failed to create server config", http.StatusInternalServerError)
		return
	}

	if ProvisioningService == nil {
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
		MachineAgentImage: cfg.MachineAgentImage,
		GCPProject:        cfg.ProjectID,
	}

	if err := ProvisioningService.CreateServer(ctx, serverID, programConfig); err != nil {
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
		Outdated:            true,
	})
}

func ListServers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dbConn, ctrlCfg, err := GetDBConnection(ctx)
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
	for _, sc := range configs {
		outdated := sc.InfraVersion < programs.CurrentInfraVersion
		outdatedMachineAgent := ctrlCfg.MachineAgentImage != sc.MachineAgentImage
		responses = append(responses, ServerResponse{
			ID:                   sc.ID,
			Config:               serverConfigToJSON(sc),
			CurrentInfraVersion:  programs.CurrentInfraVersion,
			Outdated:             outdated,
			OutdatedMachineAgent: outdatedMachineAgent,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func GetServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	serverID := vars["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}

	dbConn, ctrlCfg, err := GetDBConnection(ctx)
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

	outdatedMachineAgent := ctrlCfg.MachineAgentImage != serverConfig.MachineAgentImage

	response := ServerResponse{
		ID:                   serverID,
		Config:               serverConfigToJSON(serverConfig),
		CurrentInfraVersion:  programs.CurrentInfraVersion,
		Outdated:             outdated,
		OutdatedMachineAgent: outdatedMachineAgent,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func UpdateServer(w http.ResponseWriter, r *http.Request) {
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

	dbConn, cfg, err := GetDBConnection(ctx)
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

	if req.Region != nil {
		writeJSONError(w, "region is immutable; create a new server to change location", http.StatusBadRequest)
		return
	}
	if req.Zone != nil {
		writeJSONError(w, "zone is immutable; create a new server to change location", http.StatusBadRequest)
		return
	}

	if req.DiskSizeGB != nil && *req.DiskSizeGB < existingConfig.DiskSizeGB {
		writeJSONError(w, "disk size can only be increased", http.StatusBadRequest)
		return
	}

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
	existingConfig.MachineAgentImage = cfg.MachineAgentImage
	existingConfig.UpdatedAt = time.Now()

	if err := db.ValidateServerConfig(existingConfig); err != nil {
		writeJSONError(w, fmt.Sprintf("validation error: %s", err.Error()), http.StatusBadRequest)
		return
	}

	updateType := classifyUpdate(req, existingConfig)

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

	if ProvisioningService == nil {
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
		MachineAgentImage: cfg.MachineAgentImage,
		GCPProject:        cfg.ProjectID,
	}

	if err := ProvisioningService.UpdateServer(ctx, serverID, programConfig, updateType); err != nil {
		log.Printf("Error starting server update: %v", err)
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		if revertErr := ProvisioningService.RevertServerConfig(ctx, serverID); revertErr != nil {
			log.Printf("Error reverting config after failed update: %v", revertErr)
		}
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
		Outdated:            true,
	})
}

type UpdateType int

const (
	UpdateTypeInPlace UpdateType = iota
	UpdateTypeResize
	UpdateTypeRecreate
)

func classifyUpdate(req UpdateServerRequest, config *db.ServerConfig) int {
	if req.MinecraftVersion != nil {
		return int(UpdateTypeRecreate)
	}
	if req.MachineType != nil {
		return int(UpdateTypeResize)
	}
	return int(UpdateTypeInPlace)
}

func DeleteServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	serverID := vars["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}

	dbConn, _, err := GetDBConnection(ctx)
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

	if ProvisioningService == nil {
		writeJSONError(w, "provisioning service not available", http.StatusServiceUnavailable)
		return
	}

	if err := ProvisioningService.DestroyServer(ctx, serverID); err != nil {
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
