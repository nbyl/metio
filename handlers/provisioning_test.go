package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/nbyl/metio/db"
)

func TestCalculateProgress(t *testing.T) {
	tests := []struct {
		name     string
		status   *db.ProvisioningStatus
		expected int
	}{
		{
			name:     "No steps",
			status:   &db.ProvisioningStatus{Steps: []db.ProvisioningStep{}},
			expected: 0,
		},
		{
			name: "Completed status returns 100",
			status: &db.ProvisioningStatus{
				State: db.ProvisioningStateCompleted,
				Steps: []db.ProvisioningStep{
					{Name: "step1", Status: db.ProvisioningStateCompleted},
				},
			},
			expected: 100,
		},
		{
			name: "One of two steps completed",
			status: &db.ProvisioningStatus{
				State: db.ProvisioningStateInProgress,
				Steps: []db.ProvisioningStep{
					{Name: "step1", Status: db.ProvisioningStateCompleted},
					{Name: "step2", Status: db.ProvisioningStateInProgress},
				},
			},
			expected: 50,
		},
		{
			name: "Two of three steps completed",
			status: &db.ProvisioningStatus{
				State: db.ProvisioningStateInProgress,
				Steps: []db.ProvisioningStep{
					{Name: "step1", Status: db.ProvisioningStateCompleted},
					{Name: "step2", Status: db.ProvisioningStateCompleted},
					{Name: "step3", Status: db.ProvisioningStatePending},
				},
			},
			expected: 66,
		},
		{
			name: "No steps completed",
			status: &db.ProvisioningStatus{
				State: db.ProvisioningStateInProgress,
				Steps: []db.ProvisioningStep{
					{Name: "step1", Status: db.ProvisioningStatePending},
					{Name: "step2", Status: db.ProvisioningStatePending},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateProgress(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToProvisioningStatusResponse(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(5 * time.Minute)

	status := &db.ProvisioningStatus{
		ID:          "server-123-op",
		Operation:   db.ProvisioningOperationCreate,
		State:       db.ProvisioningStateCompleted,
		CurrentStep: "deploy_infrastructure",
		Steps: []db.ProvisioningStep{
			{Name: "create_service_account", Status: db.ProvisioningStateCompleted, Message: "Completed", Timestamp: now},
			{Name: "deploy_infrastructure", Status: db.ProvisioningStateCompleted, Message: "Completed", Timestamp: now},
		},
		StartedAt:   now,
		CompletedAt: &completedAt,
		Error:       "",
		Outputs:     map[string]string{"instanceIP": "10.0.0.1"},
	}

	response := toProvisioningStatusResponse(status)

	assert.Equal(t, "server-123-op", response.ID)
	assert.Equal(t, "CREATE", response.Operation)
	assert.Equal(t, "COMPLETED", response.State)
	assert.Equal(t, "deploy_infrastructure", response.CurrentStep)
	assert.Equal(t, 100, response.Progress)
	assert.Len(t, response.Steps, 2)
	assert.Equal(t, "create_service_account", response.Steps[0].Name)
	assert.Equal(t, "COMPLETED", response.Steps[0].Status)
	assert.NotNil(t, response.CompletedAt)
	assert.Equal(t, "10.0.0.1", response.Outputs["instanceIP"])
}

func TestToProvisioningStatusResponse_NilCompletedAt(t *testing.T) {
	now := time.Now()
	status := &db.ProvisioningStatus{
		ID:          "server-456",
		Operation:   db.ProvisioningOperationUpdate,
		State:       db.ProvisioningStateInProgress,
		CurrentStep: "deploy_infrastructure",
		Steps:       []db.ProvisioningStep{},
		StartedAt:   now,
		CompletedAt: nil,
	}

	response := toProvisioningStatusResponse(status)
	assert.Nil(t, response.CompletedAt)
	assert.Equal(t, "UPDATE", response.Operation)
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
