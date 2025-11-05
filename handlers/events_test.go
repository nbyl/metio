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

func TestDecodeBase64(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
		hasError bool
	}{
		{
			name:     "Valid base64",
			input:    []byte("SGVsbG8gV29ybGQ="),
			expected: "Hello World",
			hasError: false,
		},
		{
			name:     "Invalid base64",
			input:    []byte("invalid!"),
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodeBase64(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, string(result))
			}
		})
	}
}

func TestProcessAuditLogEvent(t *testing.T) {
	// Create a sample audit log entry
	auditLog := AuditLogEntry{
		ProtoPayload: struct {
			MethodName         string `json:"methodName"`
			ResourceName       string `json:"resourceName"`
			AuthenticationInfo struct {
				PrincipalEmail string `json:"principalEmail"`
			} `json:"authenticationInfo"`
			Request struct {
				Instance string `json:"instance"`
				Zone     string `json:"zone"`
			} `json:"request"`
		}{
			MethodName:   "compute.instances.stop",
			ResourceName: "projects/test-project/zones/us-central1-a/instances/test-instance",
		},
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
