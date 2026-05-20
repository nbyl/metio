package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/pulumi"
	"gitlab.com/nbyl/metio/pulumi/programs"
	"gitlab.com/nbyl/metio/testutil"
)

// Re-export for backward compatibility within this test file
type MockDB = testutil.MockDB

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "timeout error",
			err:      errors.New("operation timeout"),
			expected: true,
		},
		{
			name:     "rate limit",
			err:      errors.New("rate limit exceeded"),
			expected: true,
		},
		{
			name:     "quota exceeded",
			err:      errors.New("quota exceeded"),
			expected: true,
		},
		{
			name:     "502 bad gateway",
			err:      errors.New("502 bad gateway"),
			expected: true,
		},
		{
			name:     "503 service unavailable",
			err:      errors.New("503 service unavailable"),
			expected: true,
		},
		{
			name:     "504 gateway timeout",
			err:      errors.New("504 gateway timeout"),
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      errors.New("stack not found"),
			expected: false,
		},
		{
			name:     "permission denied",
			err:      errors.New("permission denied"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProvisioningState(t *testing.T) {
	assert.Equal(t, 0, int(db.ProvisioningStatePending))
	assert.Equal(t, 1, int(db.ProvisioningStateInProgress))
	assert.Equal(t, 2, int(db.ProvisioningStateCompleted))
	assert.Equal(t, 3, int(db.ProvisioningStateFailed))
}

func TestProvisioningStateString(t *testing.T) {
	assert.Equal(t, "PENDING", db.ProvisioningStatePending.String())
	assert.Equal(t, "IN_PROGRESS", db.ProvisioningStateInProgress.String())
	assert.Equal(t, "COMPLETED", db.ProvisioningStateCompleted.String())
	assert.Equal(t, "FAILED", db.ProvisioningStateFailed.String())
	assert.Equal(t, "UNKNOWN", db.ProvisioningState(99).String())
}

func TestProvisioningStateFirestoreValue(t *testing.T) {
	assert.Equal(t, "pending", db.ProvisioningStatePending.FirestoreValue())
	assert.Equal(t, "in_progress", db.ProvisioningStateInProgress.FirestoreValue())
	assert.Equal(t, "completed", db.ProvisioningStateCompleted.FirestoreValue())
	assert.Equal(t, "failed", db.ProvisioningStateFailed.FirestoreValue())
}

func TestProvisioningOperation(t *testing.T) {
	assert.Equal(t, 0, int(db.ProvisioningOperationCreate))
	assert.Equal(t, 1, int(db.ProvisioningOperationUpdate))
	assert.Equal(t, 2, int(db.ProvisioningOperationDestroy))
}

func TestProvisioningOperationString(t *testing.T) {
	assert.Equal(t, "CREATE", db.ProvisioningOperationCreate.String())
	assert.Equal(t, "UPDATE", db.ProvisioningOperationUpdate.String())
	assert.Equal(t, "DESTROY", db.ProvisioningOperationDestroy.String())
	assert.Equal(t, "UNKNOWN", db.ProvisioningOperation(99).String())
}

func TestProvisioningOperationFirestoreValue(t *testing.T) {
	assert.Equal(t, "create", db.ProvisioningOperationCreate.FirestoreValue())
	assert.Equal(t, "update", db.ProvisioningOperationUpdate.FirestoreValue())
	assert.Equal(t, "destroy", db.ProvisioningOperationDestroy.FirestoreValue())
}

func TestNewProvisioningService(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")

	service := NewProvisioningService(wm, mockDB)

	assert.NotNil(t, service)
	assert.Equal(t, 30*time.Minute, service.operationTimeout)
	assert.Equal(t, 3, service.retryAttempts)
	assert.Equal(t, 5*time.Second, service.retryDelay)
}

func TestCompleteStep(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	status := &db.ProvisioningStatus{
		Steps: []db.ProvisioningStep{
			{Name: stepUpsertStack, Status: db.ProvisioningStatePending, Message: "Preparing Pulumi stack...", Timestamp: time.Now()},
			{Name: stepDeployInfrastructure, Status: db.ProvisioningStatePending, Message: "Deploying infrastructure...", Timestamp: time.Now()},
		},
	}

	service.completeStep(status, "test-server", stepUpsertStack)

	assert.Equal(t, db.ProvisioningStateCompleted, status.Steps[0].Status)
	assert.Equal(t, "Completed", status.Steps[0].Message)
	assert.Equal(t, db.ProvisioningStatePending, status.Steps[1].Status)
	assert.Equal(t, stepUpsertStack, status.CurrentStep)
}

func TestCompleteStepNotFound(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	status := &db.ProvisioningStatus{
		Steps: []db.ProvisioningStep{
			{Name: stepUpsertStack, Status: db.ProvisioningStatePending, Message: "Preparing Pulumi stack...", Timestamp: time.Now()},
		},
	}

	service.completeStep(status, "test-server", "non_existent_step")

	assert.Equal(t, db.ProvisioningStatePending, status.Steps[0].Status)
	assert.Equal(t, "non_existent_step", status.CurrentStep)
}

func TestUpdateStep(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	service.updateStep(context.Background(), "test-server", stepDeployInfrastructure, "Deploying infrastructure...")
}

func TestHandleError(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	mockDB.On("UpdateProvisioningStatus", mock.Anything, "test-server", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	status := &db.ProvisioningStatus{
		Steps: []db.ProvisioningStep{
			{Name: stepDeployInfrastructure, Status: db.ProvisioningStatePending, Message: "Deploying infrastructure...", Timestamp: time.Now()},
		},
	}

	testErr := errors.New("deployment failed")
	err := service.handleError(status, context.Background(), "test-server", stepDeployInfrastructure, testErr)

	assert.Equal(t, testErr, err)
	assert.Equal(t, db.ProvisioningStateFailed, status.State)
	assert.Equal(t, stepDeployInfrastructure, status.CurrentStep)
	assert.Equal(t, "deployment failed", status.Error)
	assert.Equal(t, db.ProvisioningStateFailed, status.Steps[0].Status)
	assert.Equal(t, "deployment failed", status.Steps[0].Message)
}

func TestExecuteUpWithRetry_Success(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)
	service.retryAttempts = 3
	service.retryDelay = 1 * time.Millisecond

	result, err := service.executeUpWithRetry(context.Background(), func() (auto.UpResult, error) {
		return auto.UpResult{StdOut: "success"}, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "success", result.StdOut)
}

func TestExecuteUpWithRetry_NonRetryableError(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)
	service.retryAttempts = 3
	service.retryDelay = 1 * time.Millisecond

	testErr := errors.New("permission denied")
	_, err := service.executeUpWithRetry(context.Background(), func() (auto.UpResult, error) {
		return auto.UpResult{}, testErr
	})

	assert.Equal(t, testErr, err)
}

func TestExecuteUpWithRetry_RetryableError(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)
	service.retryAttempts = 3
	service.retryDelay = 1 * time.Millisecond

	testErr := errors.New("connection refused")
	_, err := service.executeUpWithRetry(context.Background(), func() (auto.UpResult, error) {
		return auto.UpResult{}, testErr
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation failed after 3 attempts")
}

func TestUpdateStatus(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	mockDB.On("UpdateProvisioningStatus", mock.Anything, "test-server", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	status := &db.ProvisioningStatus{
		ID:        "test-server-123",
		Operation: db.ProvisioningOperationCreate,
		State:     db.ProvisioningStateInProgress,
	}

	service.updateStatus(context.Background(), "test-server", status)

	mockDB.AssertExpectations(t)
}

func TestProvisioningStepStruct(t *testing.T) {
	now := time.Now()
	step := db.ProvisioningStep{
		Name:      "test_step",
		Status:    db.ProvisioningStateInProgress,
		Message:   "Processing...",
		Timestamp: now,
	}

	assert.Equal(t, "test_step", step.Name)
	assert.Equal(t, db.ProvisioningStateInProgress, step.Status)
	assert.Equal(t, "Processing...", step.Message)
	assert.Equal(t, now, step.Timestamp)
}

func TestProvisioningStatusStruct(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(5 * time.Minute)

	status := db.ProvisioningStatus{
		ID:          "server-123",
		Operation:   db.ProvisioningOperationCreate,
		State:       db.ProvisioningStateCompleted,
		StartedAt:   now,
		CompletedAt: &completedAt,
		CurrentStep: "deploy_infrastructure",
		Steps: []db.ProvisioningStep{
			{Name: "deploy_infrastructure", Status: db.ProvisioningStateCompleted, Message: "Completed", Timestamp: now},
		},
		Error: "",
		Outputs: map[string]string{
			"instanceName": "test-instance",
			"instanceIP":   "10.0.0.1",
		},
	}

	assert.Equal(t, "server-123", status.ID)
	assert.Equal(t, db.ProvisioningOperationCreate, status.Operation)
	assert.Equal(t, db.ProvisioningStateCompleted, status.State)
	assert.Equal(t, "deploy_infrastructure", status.CurrentStep)
	assert.Len(t, status.Steps, 1)
	assert.NotNil(t, status.Outputs)
	assert.Equal(t, "test-instance", status.Outputs["instanceName"])
}

func newTestService() (*ProvisioningService, *testutil.MockWorkspaceManager, *MockDB) {
	mockWM := new(testutil.MockWorkspaceManager)
	mockDB := new(MockDB)
	svc := &ProvisioningService{
		workspaceManager: mockWM,
		db:               mockDB,
		operations:       make(map[string]context.CancelFunc),
		operationTimeout: 5 * time.Second,
		retryAttempts:    1,
		retryDelay:       1 * time.Millisecond,
	}
	return svc, mockWM, mockDB
}

func TestCreateServer_Success(t *testing.T) {
	svc, mockWM, mockDB := newTestService()

	stack := &auto.Stack{}
	mockWM.On("UpsertStack", mock.Anything, "srv1", mock.AnythingOfType("func(*pulumi.Context) error")).Return(stack, nil)
	mockWM.On("SetConfig", mock.Anything, stack, "gcp:project", "", false).Return(nil)
	mockWM.On("ProjectID").Return("")
	mockWM.On("UpStack", mock.Anything, stack).Return(auto.UpResult{
		Outputs: auto.OutputMap{},
	}, nil)
	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	err := svc.CreateServer(context.Background(), "srv1", &programs.ServerConfig{Name: "test"})
	assert.NoError(t, err)

	// Wait for goroutine to complete
	time.Sleep(100 * time.Millisecond)
	mockDB.AssertCalled(t, "UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus"))
}

func TestCreateServer_UpsertError(t *testing.T) {
	svc, mockWM, mockDB := newTestService()

	mockWM.On("UpsertStack", mock.Anything, "srv1", mock.AnythingOfType("func(*pulumi.Context) error")).Return(nil, errors.New("upsert failed"))
	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	err := svc.CreateServer(context.Background(), "srv1", &programs.ServerConfig{Name: "test"})
	assert.NoError(t, err) // queueOperation returns nil immediately

	time.Sleep(100 * time.Millisecond)
}

func TestCreateServer_SetConfigError(t *testing.T) {
	svc, mockWM, mockDB := newTestService()

	stack := &auto.Stack{}
	mockWM.On("UpsertStack", mock.Anything, "srv1", mock.AnythingOfType("func(*pulumi.Context) error")).Return(stack, nil)
	mockWM.On("ProjectID").Return("")
	mockWM.On("SetConfig", mock.Anything, stack, "gcp:project", "", false).Return(errors.New("config error"))
	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	err := svc.CreateServer(context.Background(), "srv1", &programs.ServerConfig{Name: "test"})
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

func TestCreateServer_UpError(t *testing.T) {
	svc, mockWM, mockDB := newTestService()

	stack := &auto.Stack{}
	mockWM.On("UpsertStack", mock.Anything, "srv1", mock.AnythingOfType("func(*pulumi.Context) error")).Return(stack, nil)
	mockWM.On("ProjectID").Return("")
	mockWM.On("SetConfig", mock.Anything, stack, "gcp:project", "", false).Return(nil)
	mockWM.On("UpStack", mock.Anything, stack).Return(auto.UpResult{}, errors.New("deploy failed"))
	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	err := svc.CreateServer(context.Background(), "srv1", &programs.ServerConfig{Name: "test"})
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

func TestUpdateServer_Success(t *testing.T) {
	svc, mockWM, mockDB := newTestService()

	stack := &auto.Stack{}
	mockWM.On("UpsertStack", mock.Anything, "srv1", mock.AnythingOfType("func(*pulumi.Context) error")).Return(stack, nil)
	mockWM.On("ProjectID").Return("")
	mockWM.On("SetConfig", mock.Anything, stack, "gcp:project", "", false).Return(nil)
	mockWM.On("UpStack", mock.Anything, stack).Return(auto.UpResult{
		Outputs: auto.OutputMap{},
	}, nil)
	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	err := svc.UpdateServer(context.Background(), "srv1", &programs.ServerConfig{Name: "test"})
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

func TestDestroyServer_Success(t *testing.T) {
	svc, mockWM, mockDB := newTestService()

	mockWM.On("DestroyStack", mock.Anything, "srv1").Return(nil)
	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	err := svc.DestroyServer(context.Background(), "srv1")
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

func TestDestroyServer_Error(t *testing.T) {
	svc, mockWM, mockDB := newTestService()

	mockWM.On("DestroyStack", mock.Anything, "srv1").Return(errors.New("destroy failed"))
	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	err := svc.DestroyServer(context.Background(), "srv1")
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

func TestQueueOperation_AlreadyInProgress(t *testing.T) {
	svc, _, _ := newTestService()

	// Manually add an operation
	svc.mu.Lock()
	svc.operations["srv1"] = func() {}
	svc.mu.Unlock()

	err := svc.CreateServer(context.Background(), "srv1", &programs.ServerConfig{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation already in progress")
}

func TestQueueOperation_CancelledContext(t *testing.T) {
	svc, _, _ := newTestService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.CreateServer(ctx, "srv1", &programs.ServerConfig{Name: "test"})
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestCancelOperation_Success(t *testing.T) {
	svc, _, _ := newTestService()

	called := false
	svc.mu.Lock()
	svc.operations["srv1"] = func() { called = true }
	svc.mu.Unlock()

	err := svc.CancelOperation(context.Background(), "srv1")
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestCancelOperation_NotFound(t *testing.T) {
	svc, _, _ := newTestService()

	err := svc.CancelOperation(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no operation in progress")
}

func TestGetProvisioningStatus(t *testing.T) {
	svc, _, mockDB := newTestService()

	expected := &db.ProvisioningStatus{ID: "status-1", State: db.ProvisioningStateCompleted}
	mockDB.On("GetProvisioningStatus", mock.Anything, "srv1").Return(expected, nil)

	result, err := svc.GetProvisioningStatus(context.Background(), "srv1")
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestGetProvisioningStatus_Error(t *testing.T) {
	svc, _, mockDB := newTestService()

	mockDB.On("GetProvisioningStatus", mock.Anything, "srv1").Return((*db.ProvisioningStatus)(nil), errors.New("not found"))

	result, err := svc.GetProvisioningStatus(context.Background(), "srv1")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestExecuteWithRetry_Success(t *testing.T) {
	svc, _, _ := newTestService()

	err := svc.executeWithRetry(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestExecuteWithRetry_NonRetryableError(t *testing.T) {
	svc, _, _ := newTestService()

	err := svc.executeWithRetry(context.Background(), func() error {
		return errors.New("permission denied")
	})
	assert.Error(t, err)
	assert.Equal(t, "permission denied", err.Error())
}

func TestExecuteWithRetry_RetryableExhausted(t *testing.T) {
	svc, _, _ := newTestService()
	svc.retryAttempts = 2

	err := svc.executeWithRetry(context.Background(), func() error {
		return errors.New("connection refused")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation failed after 2 attempts")
}

func TestExecuteWithRetry_ContextCancelled(t *testing.T) {
	svc, _, _ := newTestService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.executeWithRetry(ctx, func() error {
		return errors.New("connection refused")
	})
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestUpdateStatus_DBError(t *testing.T) {
	svc, _, mockDB := newTestService()

	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).Return(errors.New("db error"))

	status := &db.ProvisioningStatus{ID: "test"}
	// Should not panic, just log
	assert.NotPanics(t, func() {
		svc.updateStatus(context.Background(), "srv1", status)
	})
}

func TestCreateServer_WithOutputs(t *testing.T) {
	svc, mockWM, mockDB := newTestService()

	stack := &auto.Stack{}
	mockWM.On("UpsertStack", mock.Anything, "srv1", mock.AnythingOfType("func(*pulumi.Context) error")).Return(stack, nil)
	mockWM.On("ProjectID").Return("")
	mockWM.On("SetConfig", mock.Anything, stack, "gcp:project", "", false).Return(nil)
	mockWM.On("UpStack", mock.Anything, stack).Return(auto.UpResult{
		Outputs: auto.OutputMap{
			"instanceIP": auto.OutputValue{Value: "10.0.0.1"},
		},
	}, nil)
	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil)

	err := svc.CreateServer(context.Background(), "srv1", &programs.ServerConfig{Name: "test"})
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}
