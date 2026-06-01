package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/pulumi/programs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func startServerByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "startServerByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, cfg, dbErr := getDBConnection(ctx)

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	span.SetAttributes(
		attribute.String("instance.name", serverConfig.Name),
		attribute.String("instance.zone", serverConfig.Zone),
		attribute.String("instance.project", cfg.ProjectID),
	)

	c, err := newComputeClient(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "compute_client_failed"))
		log.Print(err)
		writeJSONError(w, "failed to create compute client", http.StatusInternalServerError)
		return
	}
	defer c.Close()

	startReq := &computepb.StartInstanceRequest{
		Instance: serverConfig.Name,
		Project:  cfg.ProjectID,
		Zone:     serverConfig.Zone,
	}

	err = c.Start(ctx, startReq)
	if err != nil {
		span.SetAttributes(attribute.String("error", "start_instance_failed"))
		log.Print(err)
		writeJSONError(w, "failed to start instance", http.StatusInternalServerError)
		return
	}

	if dbErr != nil {
		span.SetAttributes(attribute.String("error", "db_connection_failed"))
		log.Printf("Error connecting to db for status update: %v", dbErr)
	} else {
		currentStatus, err := dbConn.GetStatus(ctx, serverConfig.Name)
		if err != nil {
			log.Printf("Error getting current status from db: %v", err)
			currentStatus = db.Status{
				Players: db.Players{Current: 0, Max: 20},
			}
		}
		err = dbConn.UpdateStatus(ctx, serverConfig.Name, db.Status{
			Players:     currentStatus.Players,
			Timestamp:   time.Now(),
			Uptime:      currentStatus.Uptime,
			ServerState: db.ServerStateStarting,
		})
		if err != nil {
			log.Printf("Error updating server state in db: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServerActionResponse{
		Success: true,
		State:   db.ServerStateStarting,
	})
}

func stopServerByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "stopServerByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, cfg, dbErr := getDBConnection(ctx)

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	span.SetAttributes(
		attribute.String("instance.name", serverConfig.Name),
		attribute.String("instance.zone", serverConfig.Zone),
		attribute.String("instance.project", cfg.ProjectID),
	)

	c, err := newComputeClient(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "compute_client_failed"))
		log.Print(err)
		writeJSONError(w, "failed to create compute client", http.StatusInternalServerError)
		return
	}
	defer c.Close()

	stopReq := &computepb.StopInstanceRequest{
		Instance: serverConfig.Name,
		Project:  cfg.ProjectID,
		Zone:     serverConfig.Zone,
	}

	err = c.Stop(ctx, stopReq)
	if err != nil {
		span.SetAttributes(attribute.String("error", "stop_instance_failed"))
		log.Print(err)
		writeJSONError(w, "failed to stop instance", http.StatusInternalServerError)
		return
	}

	if dbErr != nil {
		span.SetAttributes(attribute.String("error", "db_connection_failed"))
		log.Printf("Error connecting to db for status update: %v", dbErr)
	} else {
		currentStatus, err := dbConn.GetStatus(ctx, serverConfig.Name)
		if err != nil {
			log.Printf("Error getting current status from db: %v", err)
			currentStatus = db.Status{
				Players: db.Players{Current: 0, Max: 20},
			}
		}
		err = dbConn.UpdateStatus(ctx, serverConfig.Name, db.Status{
			Players:     currentStatus.Players,
			Timestamp:   time.Now(),
			Uptime:      currentStatus.Uptime,
			ServerState: db.ServerStateStopping,
		})
		if err != nil {
			log.Printf("Error updating server state in db: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServerActionResponse{
		Success: true,
		State:   db.ServerStateStopping,
	})
}

func statusByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "statusByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	span.SetAttributes(
		attribute.String("instance.name", serverConfig.Name),
	)

	playerStatus, err := dbConn.GetStatus(ctx, serverConfig.Name)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			span.SetAttributes(attribute.String("status", "not_found_treated_as_stopped"))
			playerStatus = db.Status{
				Players:     db.Players{Current: 0, Max: 20},
				Timestamp:   time.Now(),
				Uptime:      "",
				ServerState: db.ServerStateStopped,
				InstanceIP:  "",
			}
		} else {
			span.SetAttributes(attribute.String("error", "get_status_failed"))
			log.Printf("Error getting status from database: %v", err)
			writeJSONError(w, "failed to get server status", http.StatusInternalServerError)
			return
		}
	}

	ip := playerStatus.InstanceIP
	if ip == "" {
		ip = "unknown:25565"
	}

	var scheduledShutdown *string
	if playerStatus.ScheduledShutdown != nil {
		formatted := playerStatus.ScheduledShutdown.Format(time.RFC3339)
		scheduledShutdown = &formatted
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatusResponse{
		Players: PlayersJSON{
			Current: playerStatus.Players.Current,
			Max:     playerStatus.Players.Max,
		},
		Timestamp:         playerStatus.Timestamp.Format(time.RFC3339),
		Uptime:            playerStatus.Uptime,
		ServerState:       string(playerStatus.ServerState),
		InstanceIP:        ip,
		Version:           playerStatus.Version,
		WhitelistEnabled:  playerStatus.WhitelistEnabled,
		ScheduledShutdown: scheduledShutdown,
	})
}

func getWhitelistByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "getWhitelistByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	span.SetAttributes(
		attribute.String("instance.name", serverConfig.Name),
	)

	whitelistConfig, err := dbConn.GetWhitelistConfig(ctx, serverConfig.Name)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			whitelistConfig = db.WhitelistConfig{Enabled: false}
		} else {
			span.SetAttributes(attribute.String("error", "get_config_failed"))
			log.Printf("Error getting whitelist config: %v", err)
			writeJSONError(w, "failed to get whitelist config", http.StatusInternalServerError)
			return
		}
	}

	entries, err := dbConn.GetWhitelistEntries(ctx, serverConfig.Name)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_entries_failed"))
		log.Printf("Error getting whitelist entries: %v", err)
		writeJSONError(w, "failed to get whitelist entries", http.StatusInternalServerError)
		return
	}

	players := make([]WhitelistPlayer, 0, len(entries))
	for _, entry := range entries {
		players = append(players, WhitelistPlayer{
			Username: entry.Username,
			UUID:     entry.UUID,
			AddedAt:  entry.AddedAt.Format(time.RFC3339),
			AddedBy:  entry.AddedBy,
		})
	}

	span.SetAttributes(
		attribute.Bool("whitelist.enabled", whitelistConfig.Enabled),
		attribute.Int("whitelist.player_count", len(players)),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(WhitelistResponse{
		Enabled: whitelistConfig.Enabled,
		Players: players,
	})
}

func addWhitelistByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "addWhitelistByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	var req AddPlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(attribute.String("error", "invalid_request_body"))
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		span.SetAttributes(attribute.String("error", "empty_username"))
		writeJSONError(w, "username is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("username", req.Username))

	profile, err := LookupMinecraftUser(ctx, req.Username)
	if err != nil {
		span.SetAttributes(attribute.String("error", "mojang_api_failed"))
		log.Printf("Error looking up user %s: %v", req.Username, err)
		writeJSONError(w, fmt.Sprintf("failed to validate username: %v", err), http.StatusBadGateway)
		return
	}

	if profile == nil {
		span.SetAttributes(attribute.String("error", "user_not_found"))
		writeJSONError(w, fmt.Sprintf("Minecraft user '%s' not found", req.Username), http.StatusNotFound)
		return
	}

	span.SetAttributes(
		attribute.String("mojang.uuid", profile.ID),
		attribute.String("mojang.name", profile.Name),
	)

	userEmail := getUserEmail(r)
	if userEmail == "" {
		userEmail = "unknown"
	}

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	entry := db.WhitelistEntry{
		Username: profile.Name,
		UUID:     FormatUUID(profile.ID),
		AddedAt:  time.Now(),
		AddedBy:  userEmail,
	}

	if err := dbConn.AddWhitelistEntry(ctx, serverConfig.Name, entry); err != nil {
		span.SetAttributes(attribute.String("error", "add_entry_failed"))
		log.Printf("Error adding whitelist entry: %v", err)
		writeJSONError(w, "failed to add player to whitelist", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("success", "true"))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(WhitelistPlayer{
		Username: entry.Username,
		UUID:     entry.UUID,
		AddedAt:  entry.AddedAt.Format(time.RFC3339),
		AddedBy:  entry.AddedBy,
	})
}

func removeWhitelistByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "removeWhitelistByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]
	uuid := vars["uuid"]

	if uuid == "" {
		span.SetAttributes(attribute.String("error", "empty_uuid"))
		writeJSONError(w, "uuid is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("uuid", uuid))

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	if err := dbConn.RemoveWhitelistEntry(ctx, serverConfig.Name, uuid); err != nil {
		span.SetAttributes(attribute.String("error", "remove_entry_failed"))
		log.Printf("Error removing whitelist entry: %v", err)
		writeJSONError(w, "failed to remove player from whitelist", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("success", "true"))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func setWhitelistEnabledByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "setWhitelistEnabledByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	var req SetEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(attribute.String("error", "invalid_request_body"))
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Bool("enabled", req.Enabled))

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	whitelistConfig := db.WhitelistConfig{Enabled: req.Enabled}
	if err := dbConn.SetWhitelistConfig(ctx, serverConfig.Name, whitelistConfig); err != nil {
		span.SetAttributes(attribute.String("error", "set_config_failed"))
		log.Printf("Error setting whitelist config: %v", err)
		writeJSONError(w, "failed to update whitelist config", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("success", "true"))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": req.Enabled})
}

func scheduleShutdownByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "scheduleShutdownByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	var req ScheduleShutdownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(attribute.String("error", "invalid_json"))
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	shutdownTime, err := time.Parse(time.RFC3339, req.ShutdownTime)
	if err != nil {
		span.SetAttributes(attribute.String("error", "invalid_time_format"))
		writeJSONError(w, "invalid time format, expected RFC3339", http.StatusBadRequest)
		return
	}

	if shutdownTime.Before(time.Now()) {
		span.SetAttributes(attribute.String("error", "time_in_past"))
		writeJSONError(w, "shutdown time must be in the future", http.StatusBadRequest)
		return
	}

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	span.SetAttributes(
		attribute.String("instance.name", serverConfig.Name),
		attribute.String("shutdown.time", shutdownTime.Format(time.RFC3339)),
	)

	currentStatus, err := dbConn.GetStatus(ctx, serverConfig.Name)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_status_failed"))
		log.Printf("Error getting current status: %v", err)
		writeJSONError(w, "failed to get current status", http.StatusInternalServerError)
		return
	}

	currentStatus.ScheduledShutdown = &shutdownTime
	currentStatus.Timestamp = time.Now()

	err = dbConn.UpdateStatus(ctx, serverConfig.Name, currentStatus)
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		log.Printf("Error updating scheduled shutdown: %v", err)
		writeJSONError(w, "failed to schedule shutdown", http.StatusInternalServerError)
		return
	}

	formattedTime := shutdownTime.Format(time.RFC3339)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ScheduleShutdownResponse{
		Success:           true,
		ScheduledShutdown: &formattedTime,
	})
}

func cancelScheduledShutdownByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "cancelScheduledShutdownByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, _, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	span.SetAttributes(attribute.String("instance.name", serverConfig.Name))

	currentStatus, err := dbConn.GetStatus(ctx, serverConfig.Name)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_status_failed"))
		log.Printf("Error getting current status: %v", err)
		writeJSONError(w, "failed to get current status", http.StatusInternalServerError)
		return
	}

	currentStatus.ScheduledShutdown = nil
	currentStatus.Timestamp = time.Now()

	err = dbConn.UpdateStatus(ctx, serverConfig.Name, currentStatus)
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		log.Printf("Error clearing scheduled shutdown: %v", err)
		writeJSONError(w, "failed to cancel scheduled shutdown", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ScheduleShutdownResponse{
		Success:           true,
		ScheduledShutdown: nil,
	})
}

// --- Shared types and utilities (moved from server.go / whitelist.go) ---

// ComputeClient abstracts GCP Compute Instance operations for testability.
type ComputeClient interface {
	Start(ctx context.Context, req *computepb.StartInstanceRequest) error
	Stop(ctx context.Context, req *computepb.StopInstanceRequest) error
	Close() error
}

type gcpComputeClient struct {
	client *compute.InstancesClient
}

func (g *gcpComputeClient) Start(ctx context.Context, req *computepb.StartInstanceRequest) error {
	_, err := g.client.Start(ctx, req)
	return err
}

func (g *gcpComputeClient) Stop(ctx context.Context, req *computepb.StopInstanceRequest) error {
	_, err := g.client.Stop(ctx, req)
	return err
}

func (g *gcpComputeClient) Close() error {
	return g.client.Close()
}

var newComputeClient = func(ctx context.Context) (ComputeClient, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	return &gcpComputeClient{client: c}, nil
}

type ServerActionResponse struct {
	Success bool           `json:"success"`
	State   db.ServerState `json:"state"`
}

type ScheduleShutdownRequest struct {
	ShutdownTime string `json:"shutdownTime"`
}

type ScheduleShutdownResponse struct {
	Success           bool    `json:"success"`
	ScheduledShutdown *string `json:"scheduledShutdown,omitempty"`
}

type WhitelistResponse struct {
	Enabled bool              `json:"enabled"`
	Players []WhitelistPlayer `json:"players"`
}

type WhitelistPlayer struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
	AddedAt  string `json:"addedAt"`
	AddedBy  string `json:"addedBy"`
}

type AddPlayerRequest struct {
	Username string `json:"username"`
}

type SetEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func getUserEmail(r *http.Request) string {
	session, err := getSessionStore().Get(r, sessionName)
	if err != nil {
		log.Printf("getUserEmail: error retrieving session: %v", err)
		return ""
	}
	email, ok := session.Values["email"].(string)
	if !ok {
		log.Printf("getUserEmail: email not found in session (isNew=%v, keys=%v)", session.IsNew, len(session.Values))
		return ""
	}
	return email
}
