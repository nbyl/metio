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
	"gitlab.com/nbyl/metio/config"
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
	Name             string                 `json:"name"`
	Region           string                 `json:"region"`
	Zone             string                 `json:"zone"`
	MachineType      string                 `json:"machineType"`
	MinecraftVersion string                 `json:"minecraftVersion"`
	DiskSizeGB       int                    `json:"diskSizeGB"`
	ShutdownSchedule *ShutdownScheduleInput `json:"shutdownSchedule,omitempty"`
	CreatedAt        string                 `json:"createdAt"`
	UpdatedAt        string                 `json:"updatedAt"`
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
	ID     string           `json:"id"`
	Config ServerConfigJSON `json:"config"`
	Status *StatusResponse  `json:"status,omitempty"`
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
		Name:             cfg.Name,
		Region:           cfg.Region,
		Zone:             cfg.Zone,
		MachineType:      cfg.MachineType,
		MinecraftVersion: cfg.MinecraftVersion,
		DiskSizeGB:       cfg.DiskSizeGB,
		ShutdownSchedule: shutdownScheduleToInput(cfg.ShutdownSchedule),
		CreatedAt:        cfg.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        cfg.UpdatedAt.Format(time.RFC3339),
	}
}

func createServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cfg := config.Load()

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

	dbConn, err := cfg.NewDBConnection(ctx)
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
		MachineAgentImage: viper.GetString("MACHINE_AGENT_IMAGE"),
		GCPProject:        cfg.ProjectID,
		InstanceName:      serverID,
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
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ServerResponse{
		ID:     serverID,
		Config: serverConfigToJSON(serverConfig),
	})
}

func listServers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cfg := config.Load()

	dbConn, err := cfg.NewDBConnection(ctx)
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
		responses = append(responses, ServerResponse{
			ID:     cfg.ID,
			Config: serverConfigToJSON(cfg),
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

	cfg := config.Load()

	dbConn, err := cfg.NewDBConnection(ctx)
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

	response := ServerResponse{
		ID:     serverID,
		Config: serverConfigToJSON(serverConfig),
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

	cfg := config.Load()

	dbConn, err := cfg.NewDBConnection(ctx)
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

	if req.Name != nil {
		existingConfig.Name = *req.Name
	}
	if req.Region != nil {
		existingConfig.Region = *req.Region
	}
	if req.Zone != nil {
		existingConfig.Zone = *req.Zone
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

	if err := dbConn.UpdateServerConfig(ctx, serverID, existingConfig); err != nil {
		log.Printf("Error updating server config: %v", err)
		writeJSONError(w, "failed to update server config", http.StatusInternalServerError)
		return
	}

	provisioningService, err := getProvisioningService(ctx, cfg)
	if err != nil {
		log.Printf("Error creating provisioning service: %v", err)
		writeJSONError(w, "failed to create provisioning service", http.StatusInternalServerError)
		return
	}

	programConfig := &programs.ServerConfig{
		Region:            existingConfig.Region,
		Zone:              existingConfig.Zone,
		MachineType:       existingConfig.MachineType,
		MinecraftVersion:  existingConfig.MinecraftVersion,
		DiskSizeGB:        existingConfig.DiskSizeGB,
		Environment:       cfg.Environment,
		MachineAgentImage: viper.GetString("MACHINE_AGENT_IMAGE"),
		GCPProject:        cfg.ProjectID,
		InstanceName:      serverID,
	}

	if err := provisioningService.UpdateServer(ctx, serverID, programConfig); err != nil {
		log.Printf("Error starting server update: %v", err)
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		writeJSONError(w, fmt.Sprintf("failed to start update: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServerResponse{
		ID:     serverID,
		Config: serverConfigToJSON(existingConfig),
	})
}

func deleteServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	serverID := vars["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}

	cfg := config.Load()

	dbConn, err := cfg.NewDBConnection(ctx)
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

	provisioningService, err := getProvisioningService(ctx, cfg)
	if err != nil {
		log.Printf("Error creating provisioning service: %v", err)
		writeJSONError(w, "failed to create provisioning service", http.StatusInternalServerError)
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
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("server %s deletion started", serverID),
	})
}
