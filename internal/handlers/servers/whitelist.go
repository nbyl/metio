package servers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/services"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func GetWhitelistByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "getWhitelistByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	dbConn, _, err := GetDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	span.SetAttributes(
		attribute.String("instance.name", serverConfig.Name),
	)

	whitelistConfig, err := dbConn.GetWhitelistConfig(ctx, serverConfig.Name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			whitelistConfig = db.WhitelistConfig{Enabled: false}
		} else {
			span.SetAttributes(attribute.String("error", "get_config_failed"))
			log.Printf("Error getting whitelist config: %v", err)
			writeJSONError(w, "failed to get whitelist config", http.StatusInternalServerError)
			return
		}
	}

	entries, err := dbConn.GetWhitelistEntries(ctx, serverConfig.Name)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_entries_failed"))
		log.Printf("Error getting whitelist entries: %v", err)
		writeJSONError(w, "failed to get whitelist entries", http.StatusInternalServerError)
		return
	}

	players := make([]WhitelistPlayer, 0, len(entries))
	for _, entry := range entries {
		players = append(players, WhitelistPlayer{
			Username: entry.Username,
			UUID:     entry.UUID,
			AddedAt:  entry.AddedAt.Format(time.RFC3339),
			AddedBy:  entry.AddedBy,
		})
	}

	span.SetAttributes(
		attribute.Bool("whitelist.enabled", whitelistConfig.Enabled),
		attribute.Int("whitelist.player_count", len(players)),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(WhitelistResponse{
		Enabled: whitelistConfig.Enabled,
		Players: players,
	})
}

func AddWhitelistByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "addWhitelistByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	var req AddPlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(attribute.String("error", "invalid_request_body"))
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		span.SetAttributes(attribute.String("error", "empty_username"))
		writeJSONError(w, "username is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("username", req.Username))

	profile, err := LookupMinecraftUser(ctx, req.Username)
	if err != nil {
		span.SetAttributes(attribute.String("error", "mojang_api_failed"))
		log.Printf("Error looking up user %s: %v", req.Username, err)
		writeJSONError(w, fmt.Sprintf("failed to validate username: %v", err), http.StatusBadGateway)
		return
	}

	if profile == nil {
		span.SetAttributes(attribute.String("error", "user_not_found"))
		writeJSONError(w, fmt.Sprintf("Minecraft user '%s' not found", req.Username), http.StatusNotFound)
		return
	}

	span.SetAttributes(
		attribute.String("mojang.uuid", profile.ID),
		attribute.String("mojang.name", profile.Name),
	)

	userEmail := GetUserEmail(r)
	if userEmail == "" {
		userEmail = "unknown"
	}

	dbConn, _, err := GetDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	entry := db.WhitelistEntry{
		Username: profile.Name,
		UUID:     services.FormatUUID(profile.ID),
		AddedAt:  time.Now(),
		AddedBy:  userEmail,
	}

	if err := dbConn.AddWhitelistEntry(ctx, serverConfig.Name, entry); err != nil {
		span.SetAttributes(attribute.String("error", "add_entry_failed"))
		log.Printf("Error adding whitelist entry: %v", err)
		writeJSONError(w, "failed to add player to whitelist", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("success", "true"))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(WhitelistPlayer{
		Username: entry.Username,
		UUID:     entry.UUID,
		AddedAt:  entry.AddedAt.Format(time.RFC3339),
		AddedBy:  entry.AddedBy,
	})
}

func RemoveWhitelistByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "removeWhitelistByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]
	uuid := vars["uuid"]

	if uuid == "" {
		span.SetAttributes(attribute.String("error", "empty_uuid"))
		writeJSONError(w, "uuid is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("uuid", uuid))

	dbConn, _, err := GetDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	if err := dbConn.RemoveWhitelistEntry(ctx, serverConfig.Name, uuid); err != nil {
		span.SetAttributes(attribute.String("error", "remove_entry_failed"))
		log.Printf("Error removing whitelist entry: %v", err)
		writeJSONError(w, "failed to remove player from whitelist", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("success", "true"))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func SetWhitelistEnabledByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "setWhitelistEnabledByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	var req SetEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(attribute.String("error", "invalid_request_body"))
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Bool("enabled", req.Enabled))

	dbConn, _, err := GetDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	serverConfig, err := dbConn.GetServerConfig(ctx, serverID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_server_config_failed"))
		log.Printf("Error getting server config: %v", err)
		writeJSONError(w, "server not found", http.StatusNotFound)
		return
	}

	whitelistConfig := db.WhitelistConfig{Enabled: req.Enabled}
	if err := dbConn.SetWhitelistConfig(ctx, serverConfig.Name, whitelistConfig); err != nil {
		span.SetAttributes(attribute.String("error", "set_config_failed"))
		log.Printf("Error setting whitelist config: %v", err)
		writeJSONError(w, "failed to update whitelist config", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("success", "true"))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": req.Enabled})
}
