package servers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/handlers/agent"
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
	ServerID         string                      `json:"server_id"`
	ServerName       string                      `json:"server_name"`
	SnapshotID       string                      `json:"snapshot_id"`
	RepositoryPrefix string                      `json:"repository_prefix"`
	CreatedAt        string                      `json:"created_at"`
	DurationSeconds  int64                       `json:"duration_seconds"`
	FileCount        int64                       `json:"file_count"`
	RepositorySize   int64                       `json:"repository_size"`
	MinecraftVersion string                      `json:"minecraft_version"`
	Status           string                      `json:"status"`
	ServerDeletedAt  string                      `json:"server_deleted_at,omitempty"`
	RetentionUntil   string                      `json:"retention_until,omitempty"`
	SourceConfig     *backupSourceConfigResponse `json:"source_config,omitempty"`
}

type backupSourceConfigResponse struct {
	Region           string `json:"region"`
	Zone             string `json:"zone"`
	MachineType      string `json:"machine_type"`
	DiskSizeGB       int    `json:"disk_size_gb"`
	MinecraftVersion string `json:"minecraft_version"`
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
