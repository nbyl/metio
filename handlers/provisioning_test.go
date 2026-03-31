package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/nbyl/metio/db"
)

func TestServerConfigRequestStruct(t *testing.T) {
	req := ServerConfigRequest{
		Region:           "europe-west3",
		Zone:             "europe-west3-a",
		MachineType:      "e2-micro",
		MinecraftVersion: "1.21.4",
		DiskSizeGB:       20,
		BackupBucket:     "my-backup-bucket",
		RCONPassword:     "secret123",
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "europe-west3", parsed["region"])
	assert.Equal(t, "europe-west3-a", parsed["zone"])
	assert.Equal(t, "e2-micro", parsed["machineType"])
	assert.Equal(t, "1.21.4", parsed["minecraftVersion"])
	assert.Equal(t, float64(20), parsed["diskSizeGB"])
	assert.Equal(t, "my-backup-bucket", parsed["backupBucket"])
	assert.Equal(t, "secret123", parsed["rconPassword"])
}

func TestCreateServerResponseStruct(t *testing.T) {
	tests := []struct {
		name     string
		response CreateServerResponse
		expected map[string]interface{}
	}{
		{
			name: "Success response",
			response: CreateServerResponse{
				Success:  true,
				ServerID: "production-server",
			},
			expected: map[string]interface{}{
				"success":  true,
				"serverId": "production-server",
			},
		},
		{
			name: "Error response",
			response: CreateServerResponse{
				Success:  false,
				ServerID: "",
				Error:    "operation already in progress",
			},
			expected: map[string]interface{}{
				"success":  false,
				"serverId": nil,
				"error":    "operation already in progress",
			},
		},
		{
			name: "Response with operation ID",
			response: CreateServerResponse{
				Success:     true,
				ServerID:    "test-server",
				OperationID: "op-123",
			},
			expected: map[string]interface{}{
				"success":     true,
				"serverId":    "test-server",
				"operationId": "op-123",
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
			assert.Equal(t, tt.expected["serverId"], parsed["serverId"])
			if errStr, ok := tt.expected["error"].(string); ok {
				assert.Equal(t, errStr, parsed["error"])
			}
			if opID, ok := tt.expected["operationId"].(string); ok {
				assert.Equal(t, opID, parsed["operationId"])
			}
		})
	}
}

func TestProvisioningStatusResponseStruct(t *testing.T) {
	nowStr := time.Now().Format("2006-01-02T15:04:05Z07:00")

	response := ProvisioningStatusResponse{
		ID:          "server-123-1234567890",
		Operation:   "CREATE",
		State:       "IN_PROGRESS",
		CurrentStep: "deploy_infrastructure",
		Steps: []StepResponse{
			{
				Name:      "create_service_account",
				Status:    "COMPLETED",
				Message:   "Completed",
				Timestamp: nowStr,
			},
			{
				Name:      "deploy_infrastructure",
				Status:    "IN_PROGRESS",
				Message:   "Deploying infrastructure...",
				Timestamp: nowStr,
			},
		},
		Error:     "",
		StartedAt: nowStr,
		Outputs: map[string]string{
			"instanceName": "my-instance",
			"instanceIP":   "34.1.2.3",
		},
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "server-123-1234567890", parsed["id"])
	assert.Equal(t, "CREATE", parsed["operation"])
	assert.Equal(t, "IN_PROGRESS", parsed["state"])
	assert.Equal(t, "deploy_infrastructure", parsed["currentStep"])

	steps := parsed["steps"].([]interface{})
	assert.Len(t, steps, 2)

	step1 := steps[0].(map[string]interface{})
	assert.Equal(t, "create_service_account", step1["name"])
	assert.Equal(t, "COMPLETED", step1["status"])
	assert.Equal(t, "Completed", step1["message"])

	step2 := steps[1].(map[string]interface{})
	assert.Equal(t, "deploy_infrastructure", step2["name"])
	assert.Equal(t, "IN_PROGRESS", step2["status"])
	assert.Equal(t, "Deploying infrastructure...", step2["message"])

	outputs := parsed["outputs"].(map[string]interface{})
	assert.Equal(t, "my-instance", outputs["instanceName"])
	assert.Equal(t, "34.1.2.3", outputs["instanceIP"])
}

func TestStepResponseStruct(t *testing.T) {
	tests := []struct {
		name         string
		step         StepResponse
		expectedJSON map[string]interface{}
	}{
		{
			name: "Completed step",
			step: StepResponse{
				Name:      "create_disk",
				Status:    "COMPLETED",
				Message:   "Completed",
				Timestamp: "2026-03-30T10:00:00Z",
			},
			expectedJSON: map[string]interface{}{
				"name":      "create_disk",
				"status":    "COMPLETED",
				"message":   "Completed",
				"timestamp": "2026-03-30T10:00:00Z",
			},
		},
		{
			name: "Failed step",
			step: StepResponse{
				Name:      "deploy_infrastructure",
				Status:    "FAILED",
				Message:   "insufficient permissions",
				Timestamp: "2026-03-30T10:00:00Z",
			},
			expectedJSON: map[string]interface{}{
				"name":      "deploy_infrastructure",
				"status":    "FAILED",
				"message":   "insufficient permissions",
				"timestamp": "2026-03-30T10:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.step)
			assert.NoError(t, err)

			var parsed map[string]interface{}
			err = json.Unmarshal(data, &parsed)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedJSON["name"], parsed["name"])
			assert.Equal(t, tt.expectedJSON["status"], parsed["status"])
			assert.Equal(t, tt.expectedJSON["message"], parsed["message"])
		})
	}
}

func TestProvisioningStateSerialization(t *testing.T) {
	states := []struct {
		state    db.ProvisioningState
		expected string
	}{
		{db.ProvisioningStatePending, "PENDING"},
		{db.ProvisioningStateInProgress, "IN_PROGRESS"},
		{db.ProvisioningStateCompleted, "COMPLETED"},
		{db.ProvisioningStateFailed, "FAILED"},
	}

	for _, tt := range states {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestProvisioningOperationSerialization(t *testing.T) {
	operations := []struct {
		opType   db.ProvisioningOperation
		expected string
	}{
		{db.ProvisioningOperationCreate, "CREATE"},
		{db.ProvisioningOperationUpdate, "UPDATE"},
		{db.ProvisioningOperationDestroy, "DESTROY"},
	}

	for _, tt := range operations {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.opType.String())
		})
	}
}

func TestProvisioningStatusResponseWithError(t *testing.T) {
	completedAt := "2026-03-30T10:05:00Z"

	response := ProvisioningStatusResponse{
		ID:          "server-123-1234567890",
		Operation:   "CREATE",
		State:       "FAILED",
		CurrentStep: "deploy_infrastructure",
		Steps: []StepResponse{
			{
				Name:      "deploy_infrastructure",
				Status:    "FAILED",
				Message:   "deployment failed",
				Timestamp: "2026-03-30T10:00:00Z",
			},
		},
		Error:       "deployment failed",
		StartedAt:   "2026-03-30T10:00:00Z",
		CompletedAt: &completedAt,
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "FAILED", parsed["state"])
	assert.Equal(t, "deployment failed", parsed["error"])
	assert.NotNil(t, parsed["completedAt"])

	steps := parsed["steps"].([]interface{})
	step := steps[0].(map[string]interface{})
	assert.Equal(t, "FAILED", step["status"])
	assert.Equal(t, "deployment failed", step["message"])
}

func TestCreateServerResponse_JSONOmitsEmptyFields(t *testing.T) {
	response := CreateServerResponse{
		Success:  true,
		ServerID: "test-server",
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, `"error":`)
	assert.NotContains(t, jsonStr, `"operationId":`)
}

func TestProvisioningStatusResponse_WithNilOutputs(t *testing.T) {
	response := ProvisioningStatusResponse{
		ID:          "server-123",
		Operation:   "DELETE",
		State:       "COMPLETED",
		CurrentStep: "cleanup_resources",
		Steps:       []StepResponse{},
		StartedAt:   "2026-03-30T10:00:00Z",
		Outputs:     nil,
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	_, hasOutputs := parsed["outputs"]
	assert.False(t, hasOutputs, "omitempty should exclude nil outputs")
}

func TestServerConfigRequest_JSONDeserialization(t *testing.T) {
	jsonStr := `{
		"region": "us-central1",
		"zone": "us-central1-a",
		"machineType": "n2-standard-2",
		"minecraftVersion": "1.20.4",
		"diskSizeGB": 50,
		"backupBucket": "backup-bucket",
		"rconPassword": "mysecretpassword"
	}`

	var req ServerConfigRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	assert.NoError(t, err)

	assert.Equal(t, "us-central1", req.Region)
	assert.Equal(t, "us-central1-a", req.Zone)
	assert.Equal(t, "n2-standard-2", req.MachineType)
	assert.Equal(t, "1.20.4", req.MinecraftVersion)
	assert.Equal(t, 50, req.DiskSizeGB)
	assert.Equal(t, "backup-bucket", req.BackupBucket)
	assert.Equal(t, "mysecretpassword", req.RCONPassword)
}

func TestServerConfigRequest_WithMinimalFields(t *testing.T) {
	jsonStr := `{"minecraftVersion": "1.21.4"}`

	var req ServerConfigRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	assert.NoError(t, err)

	assert.Equal(t, "1.21.4", req.MinecraftVersion)
	assert.Equal(t, "", req.Region)
	assert.Equal(t, "", req.MachineType)
	assert.Equal(t, 0, req.DiskSizeGB)
}
