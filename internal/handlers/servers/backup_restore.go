package servers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/db"
)

type RestoreResponse struct {
	Operation             string   `json:"operation"`
	ServerID              string   `json:"server_id"`
	BackupID              string   `json:"backup_id"`
	SnapshotID            string   `json:"snapshot_id"`
	Warnings              []string `json:"warnings,omitempty"`
	ProvisioningStatusURL string   `json:"provisioning_status_url"`
}

// RestoreBackupByID starts an asynchronous restore of the given backup onto the
// server. The operation progress can be followed via the provisioning status
// endpoint referenced in the response.
func RestoreBackupByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	serverID := vars["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}
	backupID := vars["backupId"]
	if backupID == "" {
		writeJSONError(w, "backup id is required", http.StatusBadRequest)
		return
	}

	if ProvisioningService == nil {
		writeJSONError(w, "provisioning service not available", http.StatusServiceUnavailable)
		return
	}

	dbConn, _, err := GetDBConnection(ctx)
	if err != nil {
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	config, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	backup, err := dbConn.GetBackup(ctx, serverID, backupID)
	if err != nil {
		log.Printf("Error getting backup %s for server %s: %v", backupID, serverID, err)
		writeJSONError(w, "backup not found", http.StatusNotFound)
		return
	}
	if backup.ServerID != serverID {
		writeJSONError(w, "backup does not belong to this server", http.StatusForbidden)
		return
	}
	if backup.Status != db.BackupStatusCompleted {
		writeJSONError(w, "only completed backups can be restored", http.StatusBadRequest)
		return
	}

	versionWarning := buildVersionMismatchWarning(backup.MinecraftVersion, config.MinecraftVersion)

	if err := ProvisioningService.RestoreServer(ctx, serverID, backup, versionWarning); err != nil {
		log.Printf("Error starting restore of %s onto %s: %v", backupID, serverID, err)
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		writeJSONError(w, "failed to start restore operation", http.StatusInternalServerError)
		return
	}

	resp := RestoreResponse{
		Operation:             "restore",
		ServerID:              serverID,
		BackupID:              backup.ID,
		SnapshotID:            backup.SnapshotID,
		ProvisioningStatusURL: fmt.Sprintf("/api/servers/%s/provisioning", serverID),
	}
	if versionWarning != "" {
		resp.Warnings = []string{versionWarning}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", resp.ProvisioningStatusURL)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

func buildVersionMismatchWarning(backupVersion, serverVersion string) string {
	if backupVersion == "" || serverVersion == "" || backupVersion == serverVersion {
		return ""
	}
	return fmt.Sprintf("Backup was created with Minecraft %s but the server runs %s. "+
		"Downgrading a world can cause data loss in blocks and items added by newer versions.",
		backupVersion, serverVersion)
}
