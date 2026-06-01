package servers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/gorilla/mux"
	"gitlab.com/nbyl/metio/internal/db"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
