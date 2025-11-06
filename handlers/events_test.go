package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventsHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		authHeader     string
		body           string
		expectedStatus int
	}{
		{
			name:           "Valid POST request",
			method:         "POST",
			authHeader:     "Bearer token123",
			body:           `{"message":{"data":"dGVzdA==","messageId":"123"},"subscription":"test-sub"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid method",
			method:         "GET",
			authHeader:     "Bearer token123",
			body:           "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Missing auth header",
			method:         "POST",
			authHeader:     "",
			body:           `{"message":{"data":"dGVzdA==","messageId":"123"},"subscription":"test-sub"}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid JSON",
			method:         "POST",
			authHeader:     "Bearer token123",
			body:           `invalid json`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/events", strings.NewReader(tt.body))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			eventsHandler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestExtractInstanceName(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		expected     string
	}{
		{
			name:         "Valid resource name",
			resourceName: "projects/my-project/zones/us-central1-a/instances/my-instance",
			expected:     "my-instance",
		},
		{
			name:         "Short resource name",
			resourceName: "instances/my-instance",
			expected:     "my-instance",
		},
		{
			name:         "Empty resource name",
			resourceName: "",
			expected:     "",
		},
		{
			name:         "Invalid resource name",
			resourceName: "invalid",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInstanceName(tt.resourceName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessAuditLogEvent(t *testing.T) {
	// Create a sample audit log entry
	auditLog := AuditLogEntry{
		LogName: "projects/test-project/logs/cloudaudit.googleapis.com%2Factivity",
		ProtoPayload: struct {
			MethodName         string `json:"methodName"`
			ResourceName       string `json:"resourceName"`
			AuthenticationInfo struct {
				PrincipalEmail string `json:"principalEmail"`
			} `json:"authenticationInfo"`
		}{
			MethodName:   "v1.compute.instances.stop",
			ResourceName: "projects/test-project/zones/us-central1-a/instances/test-instance",
		},
		Resource: struct {
			Type   string `json:"type"`
			Labels struct {
				InstanceID string `json:"instance_id"`
				ProjectID  string `json:"project_id"`
				Zone       string `json:"zone"`
			} `json:"labels"`
		}{
			Type: "gce_instance",
			Labels: struct {
				InstanceID string `json:"instance_id"`
				ProjectID  string `json:"project_id"`
				Zone       string `json:"zone"`
			}{
				InstanceID: "123456789",
				ProjectID:  "test-project",
				Zone:       "us-central1-a",
			},
		},
		Timestamp: "2025-11-06T08:06:46.983678Z",
		Severity:  "NOTICE",
	}

	// Convert to JSON
	auditLogJSON, err := json.Marshal(auditLog)
	assert.NoError(t, err)

	// Test that function doesn't panic
	// Note: This test doesn't actually update the database due to missing viper config
	// In a real test environment, you would set up viper configuration
	assert.NotPanics(t, func() {
		processAuditLogEvent(auditLogJSON)
	})
}
