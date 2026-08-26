package servers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/handlers/agent"
	"github.com/nbyl/metio/internal/pulumi/programs"
)

type reportRequest struct {
	SnapshotID       string `json:"snapshotId"`
	RepositoryPrefix string `json:"repositoryPrefix"`
	DurationSeconds  int64  `json:"durationSeconds"`
	FileCount        int64  `json:"fileCount"`
	RepositorySize   int64  `json:"repositorySize"`
	MinecraftVersion string `json:"minecraftVersion"`
	Status           string `json:"status"`
}

type backupResponse struct {
	ID               string                      `json:"id"`
	ServerID         string                      `json:"serverId"`
	ServerName       string                      `json:"serverName"`
	SnapshotID       string                      `json:"snapshotId"`
	RepositoryPrefix string                      `json:"repositoryPrefix"`
	CreatedAt        string                      `json:"createdAt"`
	DurationSeconds  int64                       `json:"durationSeconds"`
	FileCount        int64                       `json:"fileCount"`
	RepositorySize   int64                       `json:"repositorySize"`
	MinecraftVersion string                      `json:"minecraftVersion"`
	Status           string                      `json:"status"`
	ServerDeletedAt  string                      `json:"serverDeletedAt,omitempty"`
	RetentionUntil   string                      `json:"retentionUntil,omitempty"`
	SourceConfig     *backupSourceConfigResponse `json:"sourceConfig,omitempty"`
}

type backupSourceConfigResponse struct {
	Region           string `json:"region"`
	Zone             string `json:"zone"`
	MachineType      string `json:"machineType"`
	DiskSizeGB       int    `json:"diskSizeGB"`
	MinecraftVersion string `json:"minecraftVersion"`
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func HandleBackupReport(w http.ResponseWriter, r *http.Request) {
	serverID := mux.Vars(r)["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}

	var req reportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	dbConn, _, err := GetDBConnection(r.Context())
	if err != nil {
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(r.Context(), serverID)
	if err != nil {
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	// Validate agent token: subject must match server config name
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSONError(w, "missing authorization header", http.StatusUnauthorized)
		return
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	subject, err := agent.VerifyToken(tokenStr)
	if err != nil {
		log.Printf("Error verifying agent token: %v", err)
		writeJSONError(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}
	if subject != serverConfig.Name {
		log.Printf("Agent token subject %q does not match server config name %q", subject, serverConfig.Name)
		writeJSONError(w, "token does not match instance", http.StatusForbidden)
		return
	}

	// Validate repository prefix
	expectedPrefix := "servers/" + serverID + "/restic/"
	if req.RepositoryPrefix != expectedPrefix {
		writeJSONError(w, "invalid repository prefix", http.StatusBadRequest)
		return
	}

	// Validate status enum
	status := req.Status
	if status != "COMPLETED" && status != "FAILED" {
		writeJSONError(w, "invalid status", http.StatusBadRequest)
		return
	}

	backup := &db.Backup{
		ID:               serverID + ":" + req.SnapshotID,
		ServerID:         serverID,
		ServerName:       serverConfig.Name,
		SnapshotID:       req.SnapshotID,
		RepositoryPrefix: req.RepositoryPrefix,
		CreatedAt:        time.Now(),
		DurationSeconds:  req.DurationSeconds,
		FileCount:        req.FileCount,
		RepositorySize:   req.RepositorySize,
		MinecraftVersion: req.MinecraftVersion,
		Status:           db.BackupStatus(status),
		SourceConfig: &db.BackupSourceConfig{
			Region:           serverConfig.Region,
			Zone:             serverConfig.Zone,
			MachineType:      serverConfig.MachineType,
			DiskSizeGB:       serverConfig.DiskSizeGB,
			MinecraftVersion: serverConfig.MinecraftVersion,
		},
	}

	if err := dbConn.UpsertBackup(r.Context(), backup); err != nil {
		log.Printf("Error upserting backup: %v", err)
		writeJSONError(w, "failed to persist backup record", http.StatusInternalServerError)
		return
	}

	writeJSONResponse(w, http.StatusCreated, backupResponse{
		ID:               backup.ID,
		ServerID:         backup.ServerID,
		ServerName:       backup.ServerName,
		SnapshotID:       backup.SnapshotID,
		RepositoryPrefix: backup.RepositoryPrefix,
		CreatedAt:        backup.CreatedAt.UTC().Format(time.RFC3339),
		DurationSeconds:  backup.DurationSeconds,
		FileCount:        backup.FileCount,
		RepositorySize:   backup.RepositorySize,
		MinecraftVersion: backup.MinecraftVersion,
		Status:           string(backup.Status),
	})
}

func ListServerBackups(w http.ResponseWriter, r *http.Request) {
	serverID := mux.Vars(r)["id"]
	if serverID == "" {
		writeJSONError(w, "server id is required", http.StatusBadRequest)
		return
	}

	dbConn, _, err := GetDBConnection(r.Context())
	if err != nil {
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	// Check server exists
	_, err = dbConn.GetServerConfig(r.Context(), serverID)
	if err != nil {
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	backups, err := dbConn.ListBackupsByServer(r.Context(), serverID)
	if err != nil {
		log.Printf("Error listing backups: %v", err)
		writeJSONError(w, "failed to list backups", http.StatusInternalServerError)
		return
	}

	var resp []backupResponse
	for _, b := range backups {
		resp = append(resp, toBackupResponse(b))
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

func ListAllBackups(w http.ResponseWriter, r *http.Request) {
	dbConn, _, err := GetDBConnection(r.Context())
	if err != nil {
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	backups, err := dbConn.ListBackups(r.Context())
	if err != nil {
		log.Printf("Error listing backups: %v", err)
		writeJSONError(w, "failed to list backups", http.StatusInternalServerError)
		return
	}

	resp := make([]backupResponse, 0, len(backups))
	for _, b := range backups {
		resp = append(resp, toBackupResponse(b))
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

func toBackupResponse(b *db.Backup) backupResponse {
	resp := backupResponse{
		ID:               b.ID,
		ServerID:         b.ServerID,
		ServerName:       b.ServerName,
		SnapshotID:       b.SnapshotID,
		RepositoryPrefix: b.RepositoryPrefix,
		CreatedAt:        b.CreatedAt.UTC().Format(time.RFC3339),
		DurationSeconds:  b.DurationSeconds,
		FileCount:        b.FileCount,
		RepositorySize:   b.RepositorySize,
		MinecraftVersion: b.MinecraftVersion,
		Status:           string(b.Status),
		ServerDeletedAt:  formatTime(b.ServerDeletedAt),
		RetentionUntil:   formatTime(b.RetentionUntil),
	}
	if b.SourceConfig != nil {
		resp.SourceConfig = &backupSourceConfigResponse{
			Region:           b.SourceConfig.Region,
			Zone:             b.SourceConfig.Zone,
			MachineType:      b.SourceConfig.MachineType,
			DiskSizeGB:       b.SourceConfig.DiskSizeGB,
			MinecraftVersion: b.SourceConfig.MinecraftVersion,
		}
	}
	return resp
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// CreateServerFromBackup creates a new server from a retained backup. The
// backup's SourceConfig provides defaults for region, zone, machine type, disk
// size and Minecraft version; the caller may override any of them. The world
// is restored from the backup's snapshot before Minecraft starts on first boot.
func CreateServerFromBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	backupID := vars["backupId"]
	if backupID == "" {
		writeJSONError(w, "backup id is required", http.StatusBadRequest)
		return
	}

	var req CreateFromBackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		writeJSONError(w, "server name is required", http.StatusBadRequest)
		return
	}

	dbConn, cfg, err := GetDBConnection(ctx)
	if err != nil {
		log.Printf("Error creating db connection: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	// Find the backup by its composite ID (serverID:snapshotID).
	backups, err := dbConn.ListBackups(ctx)
	if err != nil {
		log.Printf("Error listing backups: %v", err)
		writeJSONError(w, "failed to list backups", http.StatusInternalServerError)
		return
	}

	var backup *db.Backup
	for _, b := range backups {
		if b.ID == backupID {
			backup = b
			break
		}
	}
	if backup == nil {
		writeJSONError(w, "backup not found", http.StatusNotFound)
		return
	}
	if backup.Status != db.BackupStatusCompleted {
		writeJSONError(w, "only completed backups can be used to create a server", http.StatusBadRequest)
		return
	}

	// Derive server config from the backup's SourceConfig, allowing user overrides.
	region := req.Region
	zone := req.Zone
	machineType := req.MachineType
	minecraftVersion := req.MinecraftVersion
	diskSizeGB := req.DiskSizeGB

	if backup.SourceConfig != nil {
		if region == "" {
			region = backup.SourceConfig.Region
		}
		if zone == "" {
			zone = backup.SourceConfig.Zone
		}
		if machineType == "" {
			machineType = backup.SourceConfig.MachineType
		}
		if minecraftVersion == "" {
			minecraftVersion = backup.SourceConfig.MinecraftVersion
		}
		if diskSizeGB == 0 {
			diskSizeGB = backup.SourceConfig.DiskSizeGB
		}
	}

	// Apply defaults for any remaining empty fields.
	if region == "" {
		region = "europe-west1"
	}
	if zone == "" {
		zone = "europe-west1-b"
	}
	if machineType == "" {
		machineType = "e2-medium"
	}
	if diskSizeGB == 0 {
		diskSizeGB = 10
	}
	if minecraftVersion == "" {
		minecraftVersion = "1.21.4"
	}

	serverConfig := &db.ServerConfig{
		Name:             req.Name,
		Region:           region,
		Zone:             zone,
		MachineType:      machineType,
		MinecraftVersion: minecraftVersion,
		DiskSizeGB:       diskSizeGB,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := db.ValidateServerConfig(serverConfig); err != nil {
		writeJSONError(w, fmt.Sprintf("validation error: %s", err.Error()), http.StatusBadRequest)
		return
	}

	if !isMinecraftVersionAvailable(ctx, serverConfig.MinecraftVersion) {
		writeJSONError(w, fmt.Sprintf("validation error: minecraft version %q is not available", serverConfig.MinecraftVersion), http.StatusBadRequest)
		return
	}

	if !isMachineTypeAvailable(ctx, serverConfig.MachineType) {
		writeJSONError(w, fmt.Sprintf("validation error: machine type %q is not available", serverConfig.MachineType), http.StatusBadRequest)
		return
	}

	if !isRegionAvailable(ctx, serverConfig.Region) {
		writeJSONError(w, fmt.Sprintf("validation error: region %q is not available", serverConfig.Region), http.StatusBadRequest)
		return
	}
	if !isZoneAvailable(ctx, serverConfig.Region, serverConfig.Zone) {
		writeJSONError(w, fmt.Sprintf("validation error: zone %q is not available in region %q", serverConfig.Zone, serverConfig.Region), http.StatusBadRequest)
		return
	}

	// Check for duplicate server name before any state is written.
	if taken, err := isServerNameTaken(ctx, dbConn, serverConfig.Name, ""); err != nil {
		log.Printf("Error checking for duplicate server name: %v", err)
		writeJSONError(w, "failed to check existing servers", http.StatusInternalServerError)
		return
	} else if taken {
		writeJSONError(w, fmt.Sprintf("a server named %q already exists", serverConfig.Name), http.StatusConflict)
		return
	}

	serverID := uuid.New().String()
	serverConfig.ID = serverID
	serverConfig.MachineAgentImage = cfg.MachineAgentImage

	if err := dbConn.CreateServerConfig(ctx, serverID, serverConfig); err != nil {
		log.Printf("Error creating server config: %v", err)
		writeJSONError(w, "failed to create server config", http.StatusInternalServerError)
		return
	}

	if ProvisioningService == nil {
		writeJSONError(w, "provisioning service not available", http.StatusServiceUnavailable)
		return
	}

	token, err := agent.MintToken(req.Name)
	if err != nil {
		log.Printf("Error minting agent token: %v", err)
		writeJSONError(w, "failed to create agent token", http.StatusInternalServerError)
		return
	}

	programConfig := buildProgramConfig(serverID, serverConfig, cfg, token)
	programConfig.RestoreSnapshotID = backup.SnapshotID
	programConfig.RestoreSourcePrefix = backup.RepositoryPrefix

	if err := ProvisioningService.CreateServerFromBackup(ctx, serverID, programConfig); err != nil {
		log.Printf("Error starting server provisioning from backup: %v", err)
		if err.Error() == fmt.Sprintf("operation already in progress for server %s", serverID) {
			writeJSONError(w, "operation already in progress for this server", http.StatusConflict)
			return
		}
		writeJSONError(w, fmt.Sprintf("failed to start provisioning: %s", err.Error()), http.StatusInternalServerError)
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
