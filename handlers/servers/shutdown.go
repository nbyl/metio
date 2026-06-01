package servers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func ScheduleShutdownByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "scheduleShutdownByID")
	defer span.End()

	vars := mux.Vars(r)
	serverID := vars["id"]

	var req ScheduleShutdownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(attribute.String("error", "invalid_json"))
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	shutdownTime, err := time.Parse(time.RFC3339, req.ShutdownTime)
	if err != nil {
		span.SetAttributes(attribute.String("error", "invalid_time_format"))
		writeJSONError(w, "invalid time format, expected RFC3339", http.StatusBadRequest)
		return
	}

	if shutdownTime.Before(time.Now()) {
		span.SetAttributes(attribute.String("error", "time_in_past"))
		writeJSONError(w, "shutdown time must be in the future", http.StatusBadRequest)
		return
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

	span.SetAttributes(
		attribute.String("instance.name", serverConfig.Name),
		attribute.String("shutdown.time", shutdownTime.Format(time.RFC3339)),
	)

	currentStatus, err := dbConn.GetStatus(ctx, serverConfig.Name)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_status_failed"))
		log.Printf("Error getting current status: %v", err)
		writeJSONError(w, "failed to get current status", http.StatusInternalServerError)
		return
	}

	currentStatus.ScheduledShutdown = &shutdownTime
	currentStatus.Timestamp = time.Now()

	err = dbConn.UpdateStatus(ctx, serverConfig.Name, currentStatus)
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		log.Printf("Error updating scheduled shutdown: %v", err)
		writeJSONError(w, "failed to schedule shutdown", http.StatusInternalServerError)
		return
	}

	formattedTime := shutdownTime.Format(time.RFC3339)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ScheduleShutdownResponse{
		Success:           true,
		ScheduledShutdown: &formattedTime,
	})
}

func CancelScheduledShutdownByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("server-handler")
	ctx, span := tracer.Start(ctx, "cancelScheduledShutdownByID")
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

	span.SetAttributes(attribute.String("instance.name", serverConfig.Name))

	currentStatus, err := dbConn.GetStatus(ctx, serverConfig.Name)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_status_failed"))
		log.Printf("Error getting current status: %v", err)
		writeJSONError(w, "failed to get current status", http.StatusInternalServerError)
		return
	}

	currentStatus.ScheduledShutdown = nil
	currentStatus.Timestamp = time.Now()

	err = dbConn.UpdateStatus(ctx, serverConfig.Name, currentStatus)
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		log.Printf("Error clearing scheduled shutdown: %v", err)
		writeJSONError(w, "failed to cancel scheduled shutdown", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ScheduleShutdownResponse{
		Success:           true,
		ScheduledShutdown: nil,
	})
}
