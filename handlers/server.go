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
	ctx := context.Background()
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer c.Close()

	req := &computepb.StartInstanceRequest{
		Instance: viper.GetString("INSTANCE_NAME"),
		Project:  viper.GetString("GCP_PROJECT"),
		Zone:     viper.GetString("GCP_ZONE"),
	}

	_, err = c.Start(ctx, req)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update DB with starting status
	instanceName := viper.GetString("INSTANCE_NAME")
	environment := viper.GetString("ENVIRONMENT")
	region := viper.GetString("REGION")
	projectID := viper.GetString("GCP_PROJECT")
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)

	dbConn, err := db.NewConnection(ctx, projectID, databaseID)
	if err != nil {
		log.Printf("Error connecting to db for status update: %v", err)
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
			ServerState: "STARTING",
		})
		if err != nil {
			log.Printf("Error updating server state in db: %v", err)
		}
	}

	http.Redirect(w, r, "/server", http.StatusSeeOther)
}

func stopServerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer c.Close()

	req := &computepb.StopInstanceRequest{
		Instance: viper.GetString("INSTANCE_NAME"),
		Project:  viper.GetString("GCP_PROJECT"),
		Zone:     viper.GetString("GCP_ZONE"),
	}

	_, err = c.Stop(ctx, req)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update DB with stopping status
	instanceName := viper.GetString("INSTANCE_NAME")
	environment := viper.GetString("ENVIRONMENT")
	region := viper.GetString("REGION")
	projectID := viper.GetString("GCP_PROJECT")
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)

	dbConn, err := db.NewConnection(ctx, projectID, databaseID)
	if err != nil {
		log.Printf("Error connecting to db for status update: %v", err)
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
			ServerState: "STOPPING",
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
	instanceName := viper.GetString("INSTANCE_NAME")
	environment := viper.GetString("ENVIRONMENT")
	region := viper.GetString("REGION")
	projectID := viper.GetString("GCP_PROJECT")
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)

	// Try to get all data from DB first
	dbConn, err := db.NewConnection(ctx, projectID, databaseID)
	if err != nil {
		log.Printf("Error connecting to db: %v", err)
		return getServerStatusFromGCP(ctx, instanceName, projectID, databaseID)
	}

	playerStatus, err := dbConn.GetStatus(ctx, instanceName)
	if err != nil {
		log.Printf("Error getting status from db: %v", err)
		return getServerStatusFromGCP(ctx, instanceName, projectID, databaseID)
	}

	// If server state is not available in DB, fall back to GCP
	if playerStatus.ServerState == "" {
		log.Printf("Server state not available in DB, falling back to GCP")
		return getServerStatusFromGCP(ctx, instanceName, projectID, databaseID)
	}

	// Convert server state string to computepb.Instance_State enum
	serverState := playerStatus.ServerState

	// Get IP from GCP since it's not stored in DB
	ip, err := getInstanceIP(ctx)
	if err != nil {
		log.Printf("Error getting IP from GCP: %v", err)
		ip = "unknown:25565"
	}

	return &views.ServerStatus{
		Status:     serverState,
		Players:    playerStatus.Players.Current,
		MaxPlayers: playerStatus.Players.Max,
		Uptime:     playerStatus.Uptime,
		Version:    "TBD",
		IP:         ip,
	}, nil
}

func getServerStatusFromGCP(ctx context.Context, instanceName, projectID, databaseID string) (*views.ServerStatus, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	req := &computepb.GetInstanceRequest{
		Instance: instanceName,
		Project:  projectID,
		Zone:     viper.GetString("GCP_ZONE"),
	}

	resp, err := c.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	// Get player data from db
	dbConn, err := db.NewConnection(ctx, projectID, databaseID)
	if err != nil {
		log.Printf("Error connecting to db: %v", err)
		return &views.ServerStatus{
			Status:     *resp.Status,
			Players:    0,
			MaxPlayers: 20,
			Uptime:     "unknown",
			Version:    "TBD",
			IP:         fmt.Sprintf("%s:25565", *resp.NetworkInterfaces[0].AccessConfigs[0].NatIP),
		}, nil
	}

	playerStatus, err := dbConn.GetStatus(ctx, instanceName)
	if err != nil {
		log.Printf("Error getting status from db: %v", err)
		playerStatus = db.Status{Players: db.Players{Current: 0, Max: 20}}
	}

	return &views.ServerStatus{
		Status:     *resp.Status,
		Players:    playerStatus.Players.Current,
		MaxPlayers: playerStatus.Players.Max,
		Uptime:     playerStatus.Uptime,
		Version:    "TBD",
		IP:         fmt.Sprintf("%s:25565", *resp.NetworkInterfaces[0].AccessConfigs[0].NatIP),
	}, nil
}

func getInstanceIP(ctx context.Context) (string, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return "", err
	}
	defer c.Close()

	req := &computepb.GetInstanceRequest{
		Instance: viper.GetString("INSTANCE_NAME"),
		Project:  viper.GetString("GCP_PROJECT"),
		Zone:     viper.GetString("GCP_ZONE"),
	}

	resp, err := c.Get(ctx, req)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:25565", *resp.NetworkInterfaces[0].AccessConfigs[0].NatIP), nil
}
