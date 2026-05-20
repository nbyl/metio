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
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/db"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ComputeClient abstracts GCP Compute Instance operations for testability.
type ComputeClient interface {
	Start(ctx context.Context, req *computepb.StartInstanceRequest) error
	Stop(ctx context.Context, req *computepb.StopInstanceRequest) error
	Close() error
}

// gcpComputeClient wraps the real GCP compute client.
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

// newComputeClient is a function variable for creating compute clients. Override in tests.
var newComputeClient = func(ctx context.Context) (ComputeClient, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	return &gcpComputeClient{client: c}, nil
}

// ServerStatus represents the server status for JSON API responses
type ServerStatus struct {
	Status            db.ServerState `json:"status"`
	Players           int            `json:"players"`
	MaxPlayers        int            `json:"maxPlayers"`
	Uptime            string         `json:"uptime"`
	Version           string         `json:"version"`
	IP                string         `json:"ip"`
	WhitelistEnabled  bool           `json:"whitelistEnabled"`
	ScheduledShutdown *string        `json:"scheduledShutdown,omitempty"`
}

// ServerActionResponse represents the response for start/stop actions
type ServerActionResponse struct {
	Success bool           `json:"success"`
	State   db.ServerState `json:"state"`
}

// ScheduleShutdownRequest represents the request to schedule a shutdown
type ScheduleShutdownRequest struct {
	ShutdownTime string `json:"shutdownTime"` // RFC3339 format
}

// ScheduleShutdownResponse represents the response for scheduling a shutdown
type ScheduleShutdownResponse struct {
	Success           bool    `json:"success"`
	ScheduledShutdown *string `json:"scheduledShutdown,omitempty"`
}

// writeJSONError writes a JSON error response with the given message and status code
func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func startServerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "startServerHandler")
	defer span.End()

	dbConn, cfg, dbErr := getDBConnection(ctx)
	zone := viper.GetString("GCP_ZONE")

	span.SetAttributes(
		attribute.String("instance.name", cfg.InstanceName),
		attribute.String("instance.project", cfg.ProjectID),
		attribute.String("instance.zone", zone),
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
		Instance: cfg.InstanceName,
		Project:  cfg.ProjectID,
		Zone:     zone,
	}

	err = c.Start(ctx, startReq)
	if err != nil {
		span.SetAttributes(attribute.String("error", "start_instance_failed"))
		log.Print(err)
		writeJSONError(w, "failed to start instance", http.StatusInternalServerError)
		return
	}

	// Update DB with starting status
	if dbErr != nil {
		span.SetAttributes(attribute.String("error", "db_connection_failed"))
		log.Printf("Error connecting to db for status update: %v", dbErr)
	} else {
		// Get current status to preserve player data
		currentStatus, err := dbConn.GetStatus(ctx, cfg.InstanceName)
		if err != nil {
			log.Printf("Error getting current status from db: %v", err)
			currentStatus = db.Status{
				Players: db.Players{Current: 0, Max: 20},
			}
		}

		// Update with starting status
		err = dbConn.UpdateStatus(ctx, cfg.InstanceName, db.Status{
			Players:     currentStatus.Players,
			Timestamp:   time.Now(),
			Uptime:      currentStatus.Uptime,
			ServerState: db.ServerStateStarting,
		})
		if err != nil {
			log.Printf("Error updating server state in db: %v", err)
		}
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServerActionResponse{
		Success: true,
		State:   db.ServerStateStarting,
	})
}

func stopServerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "stopServerHandler")
	defer span.End()

	dbConn, cfg, dbErr := getDBConnection(ctx)
	zone := viper.GetString("GCP_ZONE")

	span.SetAttributes(
		attribute.String("instance.name", cfg.InstanceName),
		attribute.String("instance.project", cfg.ProjectID),
		attribute.String("instance.zone", zone),
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
		Instance: cfg.InstanceName,
		Project:  cfg.ProjectID,
		Zone:     zone,
	}

	err = c.Stop(ctx, stopReq)
	if err != nil {
		span.SetAttributes(attribute.String("error", "stop_instance_failed"))
		log.Print(err)
		writeJSONError(w, "failed to stop instance", http.StatusInternalServerError)
		return
	}

	// Update DB with stopping status
	if dbErr != nil {
		span.SetAttributes(attribute.String("error", "db_connection_failed"))
		log.Printf("Error connecting to db for status update: %v", dbErr)
	} else {
		// Get current status to preserve player data
		currentStatus, err := dbConn.GetStatus(ctx, cfg.InstanceName)
		if err != nil {
			log.Printf("Error getting current status from db: %v", err)
			currentStatus = db.Status{
				Players: db.Players{Current: 0, Max: 20},
			}
		}

		// Update with stopping status
		err = dbConn.UpdateStatus(ctx, cfg.InstanceName, db.Status{
			Players:     currentStatus.Players,
			Timestamp:   time.Now(),
			Uptime:      currentStatus.Uptime,
			ServerState: db.ServerStateStopping,
		})
		if err != nil {
			log.Printf("Error updating server state in db: %v", err)
		}
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServerActionResponse{
		Success: true,
		State:   db.ServerStateStopping,
	})
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "statusHandler")
	defer span.End()

	serverStatus, err := getServerStatus(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_status_failed"))
		log.Print(err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(serverStatus)
}

func getServerStatus(ctx context.Context) (*ServerStatus, error) {
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "getServerStatus")
	defer span.End()

	dbConn, cfg, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	span.SetAttributes(
		attribute.String("instance.name", cfg.InstanceName),
		attribute.String("database.id", cfg.DatabaseID()),
	)

	playerStatus, err := dbConn.GetStatus(ctx, cfg.InstanceName)
	if err != nil {
		// Check if this is a "not found" error (fresh deployment with no data)
		if status.Code(err) == codes.NotFound {
			// For fresh deployments, treat missing data as stopped server
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
			return nil, fmt.Errorf("error getting status from database: %w", err)
		}
	}

	// Handle missing IP with default
	ip := playerStatus.InstanceIP
	if ip == "" {
		ip = "unknown:25565"
	}

	// Only show player/uptime data when server is running
	var players, maxPlayers int
	var uptime, version string
	if playerStatus.ServerState == db.ServerStateRunning {
		players = playerStatus.Players.Current
		maxPlayers = playerStatus.Players.Max
		uptime = playerStatus.Uptime
		version = playerStatus.Version
	} else {
		players = 0
		maxPlayers = 0
		uptime = ""
		version = ""
	}

	// Format scheduled shutdown time for response
	var scheduledShutdown *string
	if playerStatus.ScheduledShutdown != nil {
		formatted := playerStatus.ScheduledShutdown.Format(time.RFC3339)
		scheduledShutdown = &formatted
	}

	return &ServerStatus{
		Status:            playerStatus.ServerState,
		Players:           players,
		MaxPlayers:        maxPlayers,
		Uptime:            uptime,
		Version:           version,
		IP:                ip,
		WhitelistEnabled:  playerStatus.WhitelistEnabled,
		ScheduledShutdown: scheduledShutdown,
	}, nil
}

func scheduleShutdownHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "scheduleShutdownHandler")
	defer span.End()

	// Parse request body
	var req ScheduleShutdownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(attribute.String("error", "invalid_json"))
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Parse shutdown time
	shutdownTime, err := time.Parse(time.RFC3339, req.ShutdownTime)
	if err != nil {
		span.SetAttributes(attribute.String("error", "invalid_time_format"))
		writeJSONError(w, "invalid time format, expected RFC3339", http.StatusBadRequest)
		return
	}

	// Validate shutdown time is in the future
	if shutdownTime.Before(time.Now()) {
		span.SetAttributes(attribute.String("error", "time_in_past"))
		writeJSONError(w, "shutdown time must be in the future", http.StatusBadRequest)
		return
	}

	// Get database connection
	dbConn, cfg, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(
		attribute.String("instance.name", cfg.InstanceName),
		attribute.String("shutdown.time", shutdownTime.Format(time.RFC3339)),
	)

	// Get current status to preserve other fields
	currentStatus, err := dbConn.GetStatus(ctx, cfg.InstanceName)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_status_failed"))
		log.Printf("Error getting current status: %v", err)
		writeJSONError(w, "failed to get current status", http.StatusInternalServerError)
		return
	}

	// Update status with scheduled shutdown
	currentStatus.ScheduledShutdown = &shutdownTime
	currentStatus.Timestamp = time.Now()

	err = dbConn.UpdateStatus(ctx, cfg.InstanceName, currentStatus)
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		log.Printf("Error updating scheduled shutdown: %v", err)
		writeJSONError(w, "failed to schedule shutdown", http.StatusInternalServerError)
		return
	}

	// Return success response
	formattedTime := shutdownTime.Format(time.RFC3339)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ScheduleShutdownResponse{
		Success:           true,
		ScheduledShutdown: &formattedTime,
	})
}

func cancelScheduledShutdownHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "cancelScheduledShutdownHandler")
	defer span.End()

	// Get database connection
	dbConn, cfg, err := getDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("instance.name", cfg.InstanceName))

	// Get current status to preserve other fields
	currentStatus, err := dbConn.GetStatus(ctx, cfg.InstanceName)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_status_failed"))
		log.Printf("Error getting current status: %v", err)
		writeJSONError(w, "failed to get current status", http.StatusInternalServerError)
		return
	}

	// Clear scheduled shutdown
	currentStatus.ScheduledShutdown = nil
	currentStatus.Timestamp = time.Now()

	err = dbConn.UpdateStatus(ctx, cfg.InstanceName, currentStatus)
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		log.Printf("Error clearing scheduled shutdown: %v", err)
		writeJSONError(w, "failed to cancel scheduled shutdown", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ScheduleShutdownResponse{
		Success:           true,
		ScheduledShutdown: nil,
	})
}
