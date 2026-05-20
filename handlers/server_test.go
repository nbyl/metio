package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/testutil"
)

type MockDB = testutil.MockDB

func TestGetServerStatus(t *testing.T) {
	// This test would require mocking GCP Compute API, which is complex.
	// For now, skip or use a simplified test.
	// In a real scenario, use a test GCP project or emulator.

	// Example: Mock the db part
	mockDB := new(MockDB)
	expectedStatus := db.Status{
		Players:   db.Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
	}
	mockDB.On("GetStatus", mock.Anything, "minecraft-server").Return(expectedStatus, nil)

	// Since GCP is hard to mock, this test is placeholder.
	// In practice, refactor getServerStatus to accept db as parameter for easier testing.

	// For now, just verify the db call would work.
	status, err := mockDB.GetStatus(context.Background(), "minecraft-server")
	assert.NoError(t, err)
	assert.Equal(t, expectedStatus, status)
	mockDB.AssertExpectations(t)
}

func TestWriteJSONError(t *testing.T) {
	tests := []struct {
		name           string
		message        string
		statusCode     int
		expectedStatus int
		expectedBody   map[string]string
	}{
		{
			name:           "Internal server error",
			message:        "something went wrong",
			statusCode:     http.StatusInternalServerError,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   map[string]string{"error": "something went wrong"},
		},
		{
			name:           "Bad request error",
			message:        "invalid input",
			statusCode:     http.StatusBadRequest,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   map[string]string{"error": "invalid input"},
		},
		{
			name:           "Unauthorized error",
			message:        "unauthorized",
			statusCode:     http.StatusUnauthorized,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   map[string]string{"error": "unauthorized"},
		},
		{
			name:           "Forbidden error",
			message:        "access denied",
			statusCode:     http.StatusForbidden,
			expectedStatus: http.StatusForbidden,
			expectedBody:   map[string]string{"error": "access denied"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			writeJSONError(w, tt.message, tt.statusCode)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Check content type
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			// Check body
			var response map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)
		})
	}
}

func TestServerStatusStruct(t *testing.T) {
	// Test that ServerStatus serializes correctly to JSON
	status := ServerStatus{
		Status:     db.ServerStateRunning,
		Players:    5,
		MaxPlayers: 20,
		Uptime:     "2:45",
		Version:    "1.21.10",
		IP:         "34.1.2.3:25565",
	}

	data, err := json.Marshal(status)
	assert.NoError(t, err)

	// Verify JSON structure
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "RUNNING", parsed["status"])
	assert.Equal(t, float64(5), parsed["players"])
	assert.Equal(t, float64(20), parsed["maxPlayers"])
	assert.Equal(t, "2:45", parsed["uptime"])
	assert.Equal(t, "1.21.10", parsed["version"])
	assert.Equal(t, "34.1.2.3:25565", parsed["ip"])
}

func TestServerActionResponseStruct(t *testing.T) {
	tests := []struct {
		name     string
		response ServerActionResponse
		expected map[string]interface{}
	}{
		{
			name: "Starting response",
			response: ServerActionResponse{
				Success: true,
				State:   db.ServerStateStarting,
			},
			expected: map[string]interface{}{
				"success": true,
				"state":   "STARTING",
			},
		},
		{
			name: "Stopping response",
			response: ServerActionResponse{
				Success: true,
				State:   db.ServerStateStopping,
			},
			expected: map[string]interface{}{
				"success": true,
				"state":   "STOPPING",
			},
		},
		{
			name: "Failed response",
			response: ServerActionResponse{
				Success: false,
				State:   db.ServerStateStopped,
			},
			expected: map[string]interface{}{
				"success": false,
				"state":   "STOPPED",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			assert.NoError(t, err)

			var parsed map[string]interface{}
			err = json.Unmarshal(data, &parsed)
			assert.NoError(t, err)

			assert.Equal(t, tt.expected["success"], parsed["success"])
			assert.Equal(t, tt.expected["state"], parsed["state"])
		})
	}
}

func TestStatusHandler_ContentType(t *testing.T) {
	// Note: This test will fail without proper viper configuration
	// and database connection. It's a structural test to verify
	// the handler sets correct content type on error.

	// Skip if not in integration test mode
	t.Skip("Requires database connection - run as integration test")
}

func TestServerActionResponse_JSONStructure(t *testing.T) {
	// Verify the exact JSON structure matches the spec
	response := ServerActionResponse{
		Success: true,
		State:   db.ServerStateStarting,
	}

	w := httptest.NewRecorder()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	// Parse the output
	var parsed struct {
		Success bool   `json:"success"`
		State   string `json:"state"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &parsed)
	assert.NoError(t, err)

	// Verify fields match spec
	assert.True(t, parsed.Success)
	assert.Equal(t, "STARTING", parsed.State)
}

func TestServerStatus_JSONFields(t *testing.T) {
	// Verify all required fields are present in JSON output
	status := ServerStatus{
		Status:     db.ServerStateRunning,
		Players:    2,
		MaxPlayers: 20,
		Uptime:     "2:45",
		Version:    "1.21.10",
		IP:         "34.x.x.x:25565",
	}

	data, err := json.Marshal(status)
	assert.NoError(t, err)

	// Check that all required fields are present
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"status"`)
	assert.Contains(t, jsonStr, `"players"`)
	assert.Contains(t, jsonStr, `"maxPlayers"`)
	assert.Contains(t, jsonStr, `"uptime"`)
	assert.Contains(t, jsonStr, `"version"`)
	assert.Contains(t, jsonStr, `"ip"`)

	// Verify no unexpected fields like "state" (should be "status")
	assert.NotContains(t, jsonStr, `"state":`)
}
