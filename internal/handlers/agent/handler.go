package agent

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
)

var GetDBConnection func(ctx context.Context) (db.DB, config.Config, error)

var StopInstance func(ctx context.Context, project, zone, instance string) error

func RegisterRoutes(agent *mux.Router) {
	agent.HandleFunc("/{instance}/status", HandleGetStatus).Methods("GET")
	agent.HandleFunc("/{instance}/status", HandleUpdateStatus).Methods("PUT")
	agent.HandleFunc("/{instance}/whitelist", HandleGetWhitelistEntries).Methods("GET")
	agent.HandleFunc("/{instance}/whitelist/config", HandleGetWhitelistConfig).Methods("GET")
	agent.HandleFunc("/{instance}/whitelist/config", HandleSetWhitelistConfig).Methods("PUT")
	agent.HandleFunc("/{instance}/whitelist", HandleAddWhitelistEntry).Methods("POST")
	agent.HandleFunc("/{instance}/stop", HandleStop).Methods("POST")
}

func HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	instance := mux.Vars(r)["instance"]
	dbConn, _, err := GetDBConnection(r.Context())
	if err != nil {
		log.Printf("[agent] db connection failed: %v", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	status, err := dbConn.GetStatus(r.Context(), instance)
	if err != nil {
		log.Printf("[agent] GetStatus(%s) failed: %v", instance, err)
		writeError(w, "status not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func HandleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	instance := mux.Vars(r)["instance"]
	var status db.Status
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	dbConn, _, err := GetDBConnection(r.Context())
	if err != nil {
		log.Printf("[agent] db connection failed: %v", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := dbConn.UpdateStatus(r.Context(), instance, status); err != nil {
		log.Printf("[agent] UpdateStatus(%s) failed: %v", instance, err)
		writeError(w, "failed to update status", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func HandleGetWhitelistEntries(w http.ResponseWriter, r *http.Request) {
	instance := mux.Vars(r)["instance"]
	dbConn, _, err := GetDBConnection(r.Context())
	if err != nil {
		log.Printf("[agent] db connection failed: %v", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	entries, err := dbConn.GetWhitelistEntries(r.Context(), instance)
	if err != nil {
		log.Printf("[agent] GetWhitelistEntries(%s) failed: %v", instance, err)
		writeError(w, "failed to get whitelist entries", http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []db.WhitelistEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func HandleGetWhitelistConfig(w http.ResponseWriter, r *http.Request) {
	instance := mux.Vars(r)["instance"]
	dbConn, _, err := GetDBConnection(r.Context())
	if err != nil {
		log.Printf("[agent] db connection failed: %v", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	config, err := dbConn.GetWhitelistConfig(r.Context(), instance)
	if err != nil {
		log.Printf("[agent] GetWhitelistConfig(%s) failed: %v", instance, err)
		writeError(w, "failed to get whitelist config", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func HandleSetWhitelistConfig(w http.ResponseWriter, r *http.Request) {
	instance := mux.Vars(r)["instance"]
	var cfg db.WhitelistConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	dbConn, _, err := GetDBConnection(r.Context())
	if err != nil {
		log.Printf("[agent] db connection failed: %v", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := dbConn.SetWhitelistConfig(r.Context(), instance, cfg); err != nil {
		log.Printf("[agent] SetWhitelistConfig(%s) failed: %v", instance, err)
		writeError(w, "failed to set whitelist config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func HandleAddWhitelistEntry(w http.ResponseWriter, r *http.Request) {
	instance := mux.Vars(r)["instance"]
	var entry db.WhitelistEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	dbConn, _, err := GetDBConnection(r.Context())
	if err != nil {
		log.Printf("[agent] db connection failed: %v", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := dbConn.AddWhitelistEntry(r.Context(), instance, entry); err != nil {
		log.Printf("[agent] AddWhitelistEntry(%s) failed: %v", instance, err)
		writeError(w, "failed to add whitelist entry", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

type StopRequest struct {
	Project string `json:"project"`
	Zone    string `json:"zone"`
}

func HandleStop(w http.ResponseWriter, r *http.Request) {
	instance := mux.Vars(r)["instance"]
	var req StopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Project == "" || req.Zone == "" {
		writeError(w, "project and zone are required", http.StatusBadRequest)
		return
	}

	if StopInstance == nil {
		log.Printf("[agent] stop handler not configured")
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := StopInstance(r.Context(), req.Project, req.Zone, instance); err != nil {
		log.Printf("[agent] StopInstance(%s) failed: %v", instance, err)
		writeError(w, "failed to stop instance", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
