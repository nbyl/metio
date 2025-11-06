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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify the request is from Pub/Sub
	if !verifyPubSubAuth(r) {
		log.Print("Unauthorized event request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse Pub/Sub message
	var pubsubMessage PubSubMessage
	if err := json.NewDecoder(r.Body).Decode(&pubsubMessage); err != nil {
		log.Printf("Invalid JSON in event request: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

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

	// Parse the audit log entry directly (data is already JSON)
	var auditLog AuditLogEntry
	if err := json.Unmarshal(data, &auditLog); err != nil {
		log.Printf("Failed to parse audit log entry: %v", err)
		log.Printf("Raw data: %s", string(data))
		return
	}

	// Process the event based on the method name
	switch auditLog.ProtoPayload.MethodName {
	case "v1.compute.instances.stop":
		handleInstanceStop(ctx, auditLog)
	case "v1.compute.instances.start":
		handleInstanceStart(ctx, auditLog)
	case "v1.compute.instances.preempted":
		handleInstancePreempted(ctx, auditLog)
	default:
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
	// Get database connection
	environment := viper.GetString("ENVIRONMENT")
	region := viper.GetString("REGION")
	projectID := viper.GetString("GCP_PROJECT")
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)

	dbConn, err := db.NewConnection(ctx, projectID, databaseID)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}

	// Get current status to preserve existing data
	currentStatus, err := dbConn.GetStatus(ctx, instanceName)
	if err != nil {
		log.Printf("Error getting current status from db: %v", err)
		// Use default values if we can't get current status
		currentStatus = db.Status{
			Players:    db.Players{Current: 0, Max: 20},
			InstanceIP: "",
		}
	}

	// Update with new state, preserving other fields
	return dbConn.UpdateStatus(ctx, instanceName, db.Status{
		Players:     currentStatus.Players,
		Timestamp:   time.Now(),
		Uptime:      currentStatus.Uptime,
		ServerState: state,
		InstanceIP:  currentStatus.InstanceIP,
	})
}
