package servers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/handlers/agent"
)

func GetBackupSettingsByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	serverID := mux.Vars(r)["id"]

	dbConn, _, err := GetDBConnection(ctx)
	if err != nil {
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(backupSettingsFromDB(serverConfig.Backup))
}

func UpdateBackupSettingsByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	serverID := mux.Vars(r)["id"]

	var req BackupSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	backupConfig := backupSettingsToDB(&req)
	if err := backupConfig.IsValid(); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	dbConn, cfg, err := GetDBConnection(ctx)
	if err != nil {
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	existingConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	originalConfig := *existingConfig
	if err := dbConn.SaveConfigSnapshot(ctx, serverID, &originalConfig); err != nil {
		log.Printf("Error saving config snapshot: %v", err)
		writeJSONError(w, "failed to save config snapshot", http.StatusInternalServerError)
		return
	}

	existingConfig.Backup = backupConfig
	existingConfig.UpdatedAt = time.Now()

	if err := dbConn.UpdateServerConfig(ctx, serverID, existingConfig); err != nil {
		log.Printf("Error updating server config: %v", err)
		writeJSONError(w, "failed to update server config", http.StatusInternalServerError)
		return
	}

	if ProvisioningService == nil {
		writeJSONError(w, "provisioning service not available", http.StatusServiceUnavailable)
		return
	}

	token, err := agent.MintToken(existingConfig.Name)
	if err != nil {
		log.Printf("Error minting agent token: %v", err)
		writeJSONError(w, "failed to create agent token", http.StatusInternalServerError)
		return
	}

	programConfig := buildProgramConfig(serverID, existingConfig, cfg, token)

	if err := ProvisioningService.UpdateServer(ctx, serverID, programConfig, int(UpdateTypeInPlace)); err != nil {
		log.Printf("Error starting server update: %v", err)
		if revertErr := ProvisioningService.RevertServerConfig(ctx, serverID); revertErr != nil {
			log.Printf("Error reverting config after failed backup update: %v", revertErr)
		}
		writeJSONError(w, "Server update failed. Your original configuration has been restored. Please try again or contact support.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/servers/"+serverID+"/provisioning")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(backupSettingsFromDB(existingConfig.Backup))
}
