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

// ServerStatus represents the server status for JSON API responses
type ServerStatus struct {
	Status           db.ServerState `json:"status"`
	Players          int            `json:"players"`
	MaxPlayers       int            `json:"maxPlayers"`
	Uptime           string         `json:"uptime"`
	Version          string         `json:"version"`
	IP               string         `json:"ip"`
	WhitelistEnabled bool           `json:"whitelistEnabled"`
}

// ServerActionResponse represents the response for start/stop actions
type ServerActionResponse struct {
	Success bool           `json:"success"`
	State   db.ServerState `json:"state"`
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

	instanceName := viper.GetString("INSTANCE_NAME")
	projectID := viper.GetString("GCP_PROJECT")
	zone := viper.GetString("GCP_ZONE")

	span.SetAttributes(
		attribute.String("instance.name", instanceName),
		attribute.String("instance.project", projectID),
		attribute.String("instance.zone", zone),
	)

	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "compute_client_failed"))
		log.Print(err)
		writeJSONError(w, "failed to create compute client", http.StatusInternalServerError)
		return
	}
	defer c.Close()

	req := &computepb.StartInstanceRequest{
		Instance: instanceName,
		Project:  projectID,
		Zone:     zone,
	}

	_, err = c.Start(ctx, req)
	if err != nil {
		span.SetAttributes(attribute.String("error", "start_instance_failed"))
		log.Print(err)
		writeJSONError(w, "failed to start instance", http.StatusInternalServerError)
		return
	}

	// Update DB with starting status
	environment := viper.GetString("ENVIRONMENT")
	region := viper.GetString("REGION")
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)

	dbConn, dbErr := db.NewConnection(ctx, projectID, databaseID)
	if dbErr != nil {
		span.SetAttributes(attribute.String("error", "db_connection_failed"))
		log.Printf("Error connecting to db for status update: %v", dbErr)
	} else {
		// Get current status to preserve player data
		currentStatus, err := dbConn.GetStatus(ctx, instanceName)
		if err != nil {
			log.Printf("Error getting current status from db: %v", err)
			currentStatus = db.Status{
				Players: db.Players{Current: 0, Max: 20},
			}
		}

		// Update with starting status
		err = dbConn.UpdateStatus(ctx, instanceName, db.Status{
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

	instanceName := viper.GetString("INSTANCE_NAME")
	projectID := viper.GetString("GCP_PROJECT")
	zone := viper.GetString("GCP_ZONE")

	span.SetAttributes(
		attribute.String("instance.name", instanceName),
		attribute.String("instance.project", projectID),
		attribute.String("instance.zone", zone),
	)

	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "compute_client_failed"))
		log.Print(err)
		writeJSONError(w, "failed to create compute client", http.StatusInternalServerError)
		return
	}
	defer c.Close()

	req := &computepb.StopInstanceRequest{
		Instance: instanceName,
		Project:  projectID,
		Zone:     zone,
	}

	_, err = c.Stop(ctx, req)
	if err != nil {
		span.SetAttributes(attribute.String("error", "stop_instance_failed"))
		log.Print(err)
		writeJSONError(w, "failed to stop instance", http.StatusInternalServerError)
		return
	}

	// Update DB with stopping status
	environment := viper.GetString("ENVIRONMENT")
	region := viper.GetString("REGION")
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)

	dbConn, dbErr := db.NewConnection(ctx, projectID, databaseID)
	if dbErr != nil {
		span.SetAttributes(attribute.String("error", "db_connection_failed"))
		log.Printf("Error connecting to db for status update: %v", dbErr)
	} else {
		// Get current status to preserve player data
		currentStatus, err := dbConn.GetStatus(ctx, instanceName)
		if err != nil {
			log.Printf("Error getting current status from db: %v", err)
			currentStatus = db.Status{
				Players: db.Players{Current: 0, Max: 20},
			}
		}

		// Update with stopping status
		err = dbConn.UpdateStatus(ctx, instanceName, db.Status{
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

	instanceName := viper.GetString("INSTANCE_NAME")
	environment := viper.GetString("ENVIRONMENT")
	region := viper.GetString("REGION")
	projectID := viper.GetString("GCP_PROJECT")
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)

	span.SetAttributes(
		attribute.String("instance.name", instanceName),
		attribute.String("database.id", databaseID),
	)

	dbConn, err := db.NewConnection(ctx, projectID, databaseID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	playerStatus, err := dbConn.GetStatus(ctx, instanceName)
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

	return &ServerStatus{
		Status:           playerStatus.ServerState,
		Players:          players,
		MaxPlayers:       maxPlayers,
		Uptime:           uptime,
		Version:          version,
		IP:               ip,
		WhitelistEnabled: playerStatus.WhitelistEnabled,
	}, nil
}
