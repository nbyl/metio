package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/db"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WhitelistResponse represents the whitelist data returned by GET /api/server/whitelist
type WhitelistResponse struct {
	Enabled bool              `json:"enabled"`
	Players []WhitelistPlayer `json:"players"`
}

// WhitelistPlayer represents a player in the whitelist response
type WhitelistPlayer struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
	AddedAt  string `json:"addedAt"`
	AddedBy  string `json:"addedBy"`
}

// AddPlayerRequest represents the request body for POST /api/server/whitelist
type AddPlayerRequest struct {
	Username string `json:"username"`
}

// SetEnabledRequest represents the request body for PUT /api/server/whitelist/enabled
type SetEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// getWhitelistHandler returns the whitelist configuration and players
func getWhitelistHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "getWhitelistHandler")
	defer span.End()

	cfg := config.Load()

	span.SetAttributes(
		attribute.String("instance.name", cfg.InstanceName),
		attribute.String("database.id", cfg.DatabaseID()),
	)

	dbConn, err := cfg.NewDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	// Get whitelist config
	whitelistConfig, err := dbConn.GetWhitelistConfig(ctx, cfg.InstanceName)
	if err != nil {
		// If not found, use default (disabled)
		if status.Code(err) == codes.NotFound {
			whitelistConfig = db.WhitelistConfig{Enabled: false}
		} else {
			span.SetAttributes(attribute.String("error", "get_config_failed"))
			log.Printf("Error getting whitelist config: %v", err)
			writeJSONError(w, "failed to get whitelist config", http.StatusInternalServerError)
			return
		}
	}

	// Get whitelist entries
	entries, err := dbConn.GetWhitelistEntries(ctx, cfg.InstanceName)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_entries_failed"))
		log.Printf("Error getting whitelist entries: %v", err)
		writeJSONError(w, "failed to get whitelist entries", http.StatusInternalServerError)
		return
	}

	// Convert to response format
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

// addWhitelistHandler adds a player to the whitelist
func addWhitelistHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "addWhitelistHandler")
	defer span.End()

	// Parse request body
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

	// Validate username via Mojang API
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

	// Get the authenticated user's email
	userEmail := getUserEmail(r)
	if userEmail == "" {
		userEmail = "unknown"
	}

	cfg := config.Load()

	dbConn, err := cfg.NewDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	// Create whitelist entry with formatted UUID
	entry := db.WhitelistEntry{
		Username: profile.Name, // Use the case-corrected name from Mojang
		UUID:     FormatUUID(profile.ID),
		AddedAt:  time.Now(),
		AddedBy:  userEmail,
	}

	if err := dbConn.AddWhitelistEntry(ctx, cfg.InstanceName, entry); err != nil {
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

// removeWhitelistHandler removes a player from the whitelist
func removeWhitelistHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "removeWhitelistHandler")
	defer span.End()

	vars := mux.Vars(r)
	uuid := vars["uuid"]

	if uuid == "" {
		span.SetAttributes(attribute.String("error", "empty_uuid"))
		writeJSONError(w, "uuid is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("uuid", uuid))

	cfg := config.Load()

	dbConn, err := cfg.NewDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	if err := dbConn.RemoveWhitelistEntry(ctx, cfg.InstanceName, uuid); err != nil {
		span.SetAttributes(attribute.String("error", "remove_entry_failed"))
		log.Printf("Error removing whitelist entry: %v", err)
		writeJSONError(w, "failed to remove player from whitelist", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("success", "true"))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// setWhitelistEnabledHandler enables or disables the whitelist
func setWhitelistEnabledHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("whitelist-handler")
	ctx, span := tracer.Start(ctx, "setWhitelistEnabledHandler")
	defer span.End()

	// Parse request body
	var req SetEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(attribute.String("error", "invalid_request_body"))
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Bool("enabled", req.Enabled))

	cfg := config.Load()

	dbConn, err := cfg.NewDBConnection(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		log.Printf("Error connecting to database: %v", err)
		writeJSONError(w, "failed to connect to database", http.StatusInternalServerError)
		return
	}

	whitelistConfig := db.WhitelistConfig{Enabled: req.Enabled}
	if err := dbConn.SetWhitelistConfig(ctx, cfg.InstanceName, whitelistConfig); err != nil {
		span.SetAttributes(attribute.String("error", "set_config_failed"))
		log.Printf("Error setting whitelist config: %v", err)
		writeJSONError(w, "failed to update whitelist config", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("success", "true"))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": req.Enabled})
}

// getUserEmail extracts the user's email from the session
func getUserEmail(r *http.Request) string {
	session, err := getSessionStore().Get(r, sessionName)
	if err != nil {
		return ""
	}
	email, ok := session.Values["email"].(string)
	if !ok {
		return ""
	}
	return email
}
