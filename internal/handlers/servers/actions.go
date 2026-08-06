package servers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/handlers/agent"
	"github.com/nbyl/metio/internal/pulumi/programs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func HandleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "handleUpdateAgent")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, cfg, err := GetDBConnection(ctx)
	if err != nil {
		log.Printf("Error creating db connection: %v", err)
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

	if ProvisioningService == nil {
		writeJSONError(w, "provisioning service not available", http.StatusServiceUnavailable)
		return
	}

	originalConfig := *serverConfig
	if err := dbConn.SaveConfigSnapshot(ctx, serverID, &originalConfig); err != nil {
		log.Printf("Error saving config snapshot: %v", err)
		writeJSONError(w, "failed to save config snapshot", http.StatusInternalServerError)
		return
	}

	serverConfig.MachineAgentImage = cfg.MachineAgentImage
	serverConfig.UpdatedAt = time.Now()

	if err := db.ValidateServerConfig(serverConfig); err != nil {
		writeJSONError(w, fmt.Sprintf("validation error: %s", err.Error()), http.StatusBadRequest)
		return
	}

	if err := dbConn.UpdateServerConfig(ctx, serverID, serverConfig); err != nil {
		log.Printf("Error updating server config: %v", err)
		writeJSONError(w, "failed to update server config", http.StatusInternalServerError)
		return
	}

	token, err := agent.MintToken(serverConfig.Name)
	if err != nil {
		log.Printf("Error minting agent token: %v", err)
		writeJSONError(w, "failed to create agent token", http.StatusInternalServerError)
		return
	}

	programConfig := &programs.ServerConfig{
		Name:              serverConfig.Name,
		ServerID:          serverID,
		Region:            serverConfig.Region,
		Zone:              serverConfig.Zone,
		MachineType:       serverConfig.MachineType,
		MinecraftVersion:  serverConfig.MinecraftVersion,
		DiskSizeGB:        serverConfig.DiskSizeGB,
		Environment:       cfg.Environment,
		MachineAgentImage: cfg.MachineAgentImage,
		GCPProject:        cfg.ProjectID,
		ExistingAddress:   serverConfig.ExistingAddress,
		ControllerURL:     cfg.BaseURL,
		AgentToken:        token,
	}

	updateType := 0
	if err := ProvisioningService.UpdateServer(ctx, serverID, programConfig, updateType); err != nil {
		log.Printf("Error starting agent update: %v", err)
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		if revertErr := ProvisioningService.RevertServerConfig(ctx, serverID); revertErr != nil {
			log.Printf("Error reverting config after failed agent update: %v", revertErr)
		}
		writeJSONError(w, "Agent update failed. Your original configuration has been restored. Please try again later.", http.StatusInternalServerError)
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
		ControllerVersion:   ControllerVersion,
	})
}

func StartServerByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "startServerByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, cfg, dbErr := GetDBConnection(ctx)

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

	c, err := NewComputeClient(ctx)
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

func StopServerByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "stopServerByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, cfg, dbErr := GetDBConnection(ctx)

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

	c, err := NewComputeClient(ctx)
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

func StatusByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "statusByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, _, err := GetDBConnection(ctx)
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
		if errors.Is(err, db.ErrNotFound) {
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
		AgentVersion:      playerStatus.AgentVersion,
	})
}
