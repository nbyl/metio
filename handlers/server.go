package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/views"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func serverHandler(w http.ResponseWriter, r *http.Request) {
	serverStatus, err := getServerStatus()
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	component := views.ServerPage(*serverStatus)
	component.Render(r.Context(), w)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	http.Redirect(w, r, "/server", http.StatusSeeOther)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	http.Redirect(w, r, "/server", http.StatusSeeOther)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	serverStatus, err := getServerStatus()
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	component := views.ServerStatusCard(*serverStatus)
	component.Render(r.Context(), w)
}

func getServerStatus() (*views.ServerStatus, error) {
	ctx := context.Background()
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
		span.SetAttributes(attribute.String("error", "get_status_failed"))
		return nil, fmt.Errorf("error getting status from database: %w", err)
	}

	// Handle missing IP with default
	ip := playerStatus.InstanceIP
	if ip == "" {
		ip = "unknown:25565"
	}

	// Only show player/uptime data when server is running
	var players, maxPlayers int
	var uptime string
	if playerStatus.ServerState == db.ServerStateRunning {
		players = playerStatus.Players.Current
		maxPlayers = playerStatus.Players.Max
		uptime = playerStatus.Uptime
	} else {
		players = 0
		maxPlayers = 0
		uptime = ""
	}

	return &views.ServerStatus{
		Status:     playerStatus.ServerState,
		Players:    players,
		MaxPlayers: maxPlayers,
		Uptime:     uptime,
		Version:    "TBD",
		IP:         ip,
	}, nil
}
