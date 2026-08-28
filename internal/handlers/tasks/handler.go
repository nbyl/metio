package tasks

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/handlers/agent"
	"github.com/nbyl/metio/internal/handlers/servers"
	"github.com/nbyl/metio/internal/pulumi/programs"
	"github.com/nbyl/metio/internal/services"
)

var ProvisioningService *services.ProvisioningService

var executeOperation = func(ctx context.Context, serverID string, programConfig *programs.ServerConfig, updateType int) error {
	return ProvisioningService.ExecuteOperation(ctx, serverID, programConfig, updateType)
}

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

	if opID := r.URL.Query().Get("opId"); opID != "" {
		current, err := dbConn.GetProvisioningStatus(ctx, serverID)
		if err != nil {
			log.Printf("[tasks] Failed to read provisioning status for %s: %v", serverID, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if current == nil || current.ID != opID {
			log.Printf("[tasks] Skipping stale/superseded provisioning task for %s (op %s, current %v)", serverID, opID, current)
			w.WriteHeader(http.StatusOK)
			return
		}
		if current.State != db.ProvisioningStateInProgress {
			log.Printf("[tasks] Skipping provisioning task for %s: operation %s is no longer in progress (state %s)", serverID, opID, current.State)
			w.WriteHeader(http.StatusOK)
			return
		}
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

	programConfig := buildProgramConfig(serverConfig, cfg, token)

	log.Printf("[tasks] Executing provisioning for server %s", serverID)

	if err := executeOperation(ctx, serverID, programConfig, 0); err != nil {
		log.Printf("[tasks] Provisioning failed for %s: %v", serverID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// buildProgramConfig assembles the Pulumi program config for a provisioning
// task from the persisted server config plus controller-level settings. It
// must stay in sync with servers.buildProgramConfig so task-executed
// provisioning rolls out the same cloud-config (backup image, per-server
// backup overrides) as the synchronous request paths.
func buildProgramConfig(serverConfig *db.ServerConfig, cfg config.Config, token string) *programs.ServerConfig {
	return &programs.ServerConfig{
		Name:                     serverConfig.Name,
		ServerID:                 serverConfig.ID,
		Region:                   serverConfig.Region,
		Zone:                     serverConfig.Zone,
		MachineType:              serverConfig.MachineType,
		MinecraftVersion:         serverConfig.MinecraftVersion,
		DiskSizeGB:               serverConfig.DiskSizeGB,
		Environment:              cfg.Environment,
		MachineAgentImage:        cfg.MachineAgentImage,
		BackupImage:              cfg.BackupImage,
		GCPProject:               cfg.ProjectID,
		ExistingAddress:          serverConfig.ExistingAddress,
		ControllerURL:            cfg.BaseURL,
		AgentToken:               token,
		Backup:                   servers.DBBackupToProgramBackup(serverConfig.Backup),
		BackupResticPassword:     cfg.BackupResticPassword,
		RetainLegacyBackupBucket: serverConfig.InfraVersion > 0 && serverConfig.InfraVersion < programs.CurrentInfraVersion,
	}
}
