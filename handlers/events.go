package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// PubSubMessage represents the structure of a Pub/Sub push message
type PubSubMessage struct {
	Message struct {
		Data []byte `json:"data"`
		ID   string `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// AuditLogEntry represents the structure of a GCP audit log entry
type AuditLogEntry struct {
	LogName      string `json:"logName"`
	ProtoPayload struct {
		MethodName         string `json:"methodName"`
		ResourceName       string `json:"resourceName"`
		AuthenticationInfo struct {
			PrincipalEmail string `json:"principalEmail"`
		} `json:"authenticationInfo"`
	} `json:"protoPayload"`
	Resource struct {
		Type   string `json:"type"`
		Labels struct {
			InstanceID string `json:"instance_id"`
			ProjectID  string `json:"project_id"`
			Zone       string `json:"zone"`
		} `json:"labels"`
	} `json:"resource"`
	Timestamp string `json:"timestamp"`
	Severity  string `json:"severity"`
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("events-handler")
	ctx, span := tracer.Start(ctx, "eventsHandler")
	defer span.End()

	// Record request metric
	tracing.RecordRequest(r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		span.SetAttributes(attribute.String("error", "method_not_allowed"))
		tracing.RecordError("method_not_allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify the request is from Pub/Sub
	if !verifyPubSubAuth(r) {
		span.SetAttributes(attribute.String("error", "unauthorized"))
		tracing.RecordError("unauthorized")
		log.Print("Unauthorized event request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse Pub/Sub message
	var pubsubMessage PubSubMessage
	if err := json.NewDecoder(r.Body).Decode(&pubsubMessage); err != nil {
		span.SetAttributes(attribute.String("error", "invalid_json"))
		tracing.RecordError("invalid_json")
		log.Printf("Invalid JSON in event request: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("pubsub.message_id", pubsubMessage.Message.ID),
		attribute.String("pubsub.subscription", pubsubMessage.Subscription),
	)

	// Process the audit log event asynchronously
	go processAuditLogEvent(pubsubMessage.Message.Data)

	// Return success immediately to Pub/Sub
	w.WriteHeader(http.StatusOK)
}

func verifyPubSubAuth(r *http.Request) bool {
	// Check for the Pub/Sub specific headers
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	// For now, we'll accept any Bearer token since Pub/Sub handles the authentication
	// In production, you might want to validate the JWT token
	return strings.HasPrefix(authHeader, "Bearer ")
}

func processAuditLogEvent(data []byte) {
	ctx := context.Background()
	tracer := otel.Tracer("events-handler")
	ctx, span := tracer.Start(ctx, "processAuditLogEvent")
	defer span.End()

	// Parse the audit log entry directly (data is already JSON)
	var auditLog AuditLogEntry
	if err := json.Unmarshal(data, &auditLog); err != nil {
		span.SetAttributes(
			attribute.String("error", "failed_to_parse_audit_log"),
			attribute.String("raw_data", string(data)),
		)
		tracing.RecordError("failed_to_parse_audit_log")
		log.Printf("Failed to parse audit log entry: %v", err)
		log.Printf("Raw data: %s", string(data))
		return
	}

	span.SetAttributes(
		attribute.String("audit.method", auditLog.ProtoPayload.MethodName),
		attribute.String("audit.resource", auditLog.ProtoPayload.ResourceName),
		attribute.String("audit.principal", auditLog.ProtoPayload.AuthenticationInfo.PrincipalEmail),
		attribute.String("audit.instance_id", auditLog.Resource.Labels.InstanceID),
		attribute.String("audit.project_id", auditLog.Resource.Labels.ProjectID),
		attribute.String("audit.zone", auditLog.Resource.Labels.Zone),
	)

	// Record event processed metric
	tracing.RecordEventProcessed(auditLog.ProtoPayload.MethodName)

	// Process the event based on the method name
	switch auditLog.ProtoPayload.MethodName {
	case "v1.compute.instances.stop":
		handleInstanceStop(ctx, auditLog)
	case "v1.compute.instances.start":
		handleInstanceStart(ctx, auditLog)
	case "v1.compute.instances.preempted":
		handleInstancePreempted(ctx, auditLog)
	default:
		span.SetAttributes(attribute.String("error", "unhandled_method"))
		log.Printf("Ignoring audit log method: %s", auditLog.ProtoPayload.MethodName)
	}
}

func handleInstanceStop(ctx context.Context, auditLog AuditLogEntry) {
	log.Printf("Handling instance stop event for: %s", auditLog.ProtoPayload.ResourceName)

	// Extract instance name from resource name
	instanceName := extractInstanceName(auditLog.ProtoPayload.ResourceName)
	if instanceName == "" {
		log.Printf("Could not extract instance name from: %s", auditLog.ProtoPayload.ResourceName)
		return
	}

	// Update database with STOPPED state
	if err := updateInstanceState(ctx, instanceName, db.ServerStateStopped); err != nil {
		log.Printf("Failed to update instance state to STOPPED: %v", err)
		return
	}

	log.Printf("Successfully updated instance %s state to STOPPED", instanceName)
}

func handleInstanceStart(ctx context.Context, auditLog AuditLogEntry) {
	log.Printf("Handling instance start event for: %s", auditLog.ProtoPayload.ResourceName)

	// Extract instance name from resource name
	instanceName := extractInstanceName(auditLog.ProtoPayload.ResourceName)
	if instanceName == "" {
		log.Printf("Could not extract instance name from: %s", auditLog.ProtoPayload.ResourceName)
		return
	}

	// Update database with STARTING state (machine-agent will update to RUNNING)
	if err := updateInstanceState(ctx, instanceName, db.ServerStateStarting); err != nil {
		log.Printf("Failed to update instance state to STARTING: %v", err)
		return
	}

	log.Printf("Successfully updated instance %s state to STARTING", instanceName)
}

func handleInstancePreempted(ctx context.Context, auditLog AuditLogEntry) {
	log.Printf("Handling instance preempted event for: %s", auditLog.ProtoPayload.ResourceName)

	// Extract instance name from resource name
	instanceName := extractInstanceName(auditLog.ProtoPayload.ResourceName)
	if instanceName == "" {
		log.Printf("Could not extract instance name from: %s", auditLog.ProtoPayload.ResourceName)
		return
	}

	// Update database with STOPPED state (preemption is effectively a stop)
	if err := updateInstanceState(ctx, instanceName, db.ServerStateStopped); err != nil {
		log.Printf("Failed to update instance state to STOPPED (preempted): %v", err)
		return
	}

	log.Printf("Successfully updated instance %s state to STOPPED (preempted)", instanceName)
}

func extractInstanceName(resourceName string) string {
	// Resource name format: projects/PROJECT_ID/zones/ZONE/instances/INSTANCE_NAME
	// Or: instances/INSTANCE_NAME
	parts := strings.Split(resourceName, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

func updateInstanceState(ctx context.Context, instanceName string, state db.ServerState) error {
	tracer := otel.Tracer("events-handler")
	ctx, span := tracer.Start(ctx, "updateInstanceState")
	defer span.End()

	span.SetAttributes(
		attribute.String("instance.name", instanceName),
		attribute.String("instance.state", string(state)),
	)

	// Record database operation metric
	tracing.RecordDBOperation("update_instance_state")

	// Get database connection
	environment := viper.GetString("ENVIRONMENT")
	region := viper.GetString("REGION")
	projectID := viper.GetString("GCP_PROJECT")
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)

	dbConn, err := db.NewConnection(ctx, projectID, databaseID)
	if err != nil {
		span.SetAttributes(attribute.String("error", "database_connection_failed"))
		tracing.RecordError("database_connection_failed")
		return fmt.Errorf("error connecting to database: %w", err)
	}

	// Get current status to preserve existing data
	currentStatus, err := dbConn.GetStatus(ctx, instanceName)
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_status_failed"))
		tracing.RecordError("get_status_failed")
		log.Printf("Error getting current status from db: %v", err)
		// Use default values if we can't get current status
		currentStatus = db.Status{
			Players:    db.Players{Current: 0, Max: 20},
			InstanceIP: "",
		}
	}

	// Update with new state, preserving other fields
	err = dbConn.UpdateStatus(ctx, instanceName, db.Status{
		Players:     currentStatus.Players,
		Timestamp:   time.Now(),
		Uptime:      currentStatus.Uptime,
		ServerState: state,
		InstanceIP:  currentStatus.InstanceIP,
	})
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		tracing.RecordError("update_status_failed")
		return err
	}

	// Record status update metric
	tracing.RecordStatusUpdate(instanceName, string(state))

	span.SetAttributes(attribute.String("success", "true"))
	return nil
}
