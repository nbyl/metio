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
)

type MockDB struct {
	mock.Mock
}

func (m *MockDB) UpdateStatus(ctx context.Context, instanceName string, status db.Status) error {
	args := m.Called(ctx, instanceName, status)
	return args.Error(0)
}

func (m *MockDB) GetStatus(ctx context.Context, instanceName string) (db.Status, error) {
	args := m.Called(ctx, instanceName)
	return args.Get(0).(db.Status), args.Error(1)
}

func (m *MockDB) GetWhitelistConfig(ctx context.Context, instanceName string) (db.WhitelistConfig, error) {
	args := m.Called(ctx, instanceName)
	return args.Get(0).(db.WhitelistConfig), args.Error(1)
}

func (m *MockDB) SetWhitelistConfig(ctx context.Context, instanceName string, config db.WhitelistConfig) error {
	args := m.Called(ctx, instanceName, config)
	return args.Error(0)
}

func (m *MockDB) GetWhitelistEntries(ctx context.Context, instanceName string) ([]db.WhitelistEntry, error) {
	args := m.Called(ctx, instanceName)
	return args.Get(0).([]db.WhitelistEntry), args.Error(1)
}

func (m *MockDB) AddWhitelistEntry(ctx context.Context, instanceName string, entry db.WhitelistEntry) error {
	args := m.Called(ctx, instanceName, entry)
	return args.Error(0)
}

func (m *MockDB) RemoveWhitelistEntry(ctx context.Context, instanceName string, uuid string) error {
	args := m.Called(ctx, instanceName, uuid)
	return args.Error(0)
}

func (m *MockDB) SetWhitelistEntries(ctx context.Context, instanceName string, entries []db.WhitelistEntry) error {
	args := m.Called(ctx, instanceName, entries)
	return args.Error(0)
}

func (m *MockDB) GetProvisioningStatus(ctx context.Context, serverID string) (*db.ProvisioningStatus, error) {
	args := m.Called(ctx, serverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.ProvisioningStatus), args.Error(1)
}

func (m *MockDB) UpdateProvisioningStatus(ctx context.Context, serverID string, status *db.ProvisioningStatus) error {
	args := m.Called(ctx, serverID, status)
	return args.Error(0)
}

func (m *MockDB) AddProvisioningStep(ctx context.Context, serverID string, step db.ProvisioningStep) error {
	args := m.Called(ctx, serverID, step)
	return args.Error(0)
}

func (m *MockDB) CompleteProvisioning(ctx context.Context, serverID string, outputs map[string]string) error {
	args := m.Called(ctx, serverID, outputs)
	return args.Error(0)
}

func (m *MockDB) FailProvisioning(ctx context.Context, serverID string, errMsg string) error {
	args := m.Called(ctx, serverID, errMsg)
	return args.Error(0)
}

func (m *MockDB) CreateServerConfig(ctx context.Context, serverID string, config *db.ServerConfig) error {
	args := m.Called(ctx, serverID, config)
	return args.Error(0)
}

func (m *MockDB) GetServerConfig(ctx context.Context, serverID string) (*db.ServerConfig, error) {
	args := m.Called(ctx, serverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.ServerConfig), args.Error(1)
}

func (m *MockDB) UpdateServerConfig(ctx context.Context, serverID string, config *db.ServerConfig) error {
	args := m.Called(ctx, serverID, config)
	return args.Error(0)
}

func (m *MockDB) DeleteServerConfig(ctx context.Context, serverID string) error {
	args := m.Called(ctx, serverID)
	return args.Error(0)
}

func (m *MockDB) ListServerConfigs(ctx context.Context) ([]*db.ServerConfig, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.ServerConfig), args.Error(1)
}

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
			{Name: stepCreateServiceAccount, Status: db.ProvisioningStatePending, Message: "Creating service account...", Timestamp: time.Now()},
			{Name: stepDeployInfrastructure, Status: db.ProvisioningStatePending, Message: "Deploying infrastructure...", Timestamp: time.Now()},
		},
	}

	service.completeStep(status, "test-server", stepCreateServiceAccount)

	assert.Equal(t, db.ProvisioningStateCompleted, status.Steps[0].Status)
	assert.Equal(t, "Completed", status.Steps[0].Message)
	assert.Equal(t, db.ProvisioningStatePending, status.Steps[1].Status)
	assert.Equal(t, stepCreateServiceAccount, status.CurrentStep)
}

func TestCompleteStepNotFound(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	status := &db.ProvisioningStatus{
		Steps: []db.ProvisioningStep{
			{Name: stepCreateServiceAccount, Status: db.ProvisioningStatePending, Message: "Creating service account...", Timestamp: time.Now()},
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
