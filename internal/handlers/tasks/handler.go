package tasks

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/handlers/agent"
	"github.com/nbyl/metio/internal/pulumi/programs"
	"github.com/nbyl/metio/internal/services"
)

var ProvisioningService *services.ProvisioningService

var getDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, config.Config{}, err
	}
	dbConn, err := cfg.NewDBConnection(ctx)
	return dbConn, cfg, err
}

func HandleProvisioningTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	serverID := vars["id"]
	if serverID == "" {
		http.Error(w, "server id is required", http.StatusBadRequest)
		return
	}

	dbConn, cfg, err := getDBConnection(ctx)
	if err != nil {
		log.Printf("[tasks] Failed to create db connection: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		log.Printf("[tasks] Server config not found for %s: %v", serverID, err)
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}

	token, err := agent.MintToken(serverConfig.Name)
	if err != nil {
		log.Printf("[tasks] Failed to mint agent token for %s: %v", serverID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
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

	log.Printf("[tasks] Executing provisioning for server %s", serverID)

	if err := ProvisioningService.ExecuteOperation(ctx, serverID, programConfig, 0); err != nil {
		log.Printf("[tasks] Provisioning failed for %s: %v", serverID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
