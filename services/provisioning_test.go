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

func (m *MockDB) GetOperation(ctx context.Context, instanceName string) (*db.Operation, error) {
	args := m.Called(ctx, instanceName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.Operation), args.Error(1)
}

func (m *MockDB) UpdateOperation(ctx context.Context, instanceName string, op *db.Operation) error {
	args := m.Called(ctx, instanceName, op)
	return args.Error(0)
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

func TestOperationState(t *testing.T) {
	assert.Equal(t, 0, int(db.OperationStatePending))
	assert.Equal(t, 1, int(db.OperationStateRunning))
	assert.Equal(t, 2, int(db.OperationStateCompleted))
	assert.Equal(t, 3, int(db.OperationStateFailed))
	assert.Equal(t, 4, int(db.OperationStateCancelled))
}

func TestOperationStateString(t *testing.T) {
	assert.Equal(t, "PENDING", db.OperationStatePending.String())
	assert.Equal(t, "RUNNING", db.OperationStateRunning.String())
	assert.Equal(t, "COMPLETED", db.OperationStateCompleted.String())
	assert.Equal(t, "FAILED", db.OperationStateFailed.String())
	assert.Equal(t, "CANCELLED", db.OperationStateCancelled.String())
	assert.Equal(t, "UNKNOWN", db.OperationState(99).String())
}

func TestOperationType(t *testing.T) {
	assert.Equal(t, 0, int(db.OperationTypeCreate))
	assert.Equal(t, 1, int(db.OperationTypeUpdate))
	assert.Equal(t, 2, int(db.OperationTypeDelete))
}

func TestOperationTypeString(t *testing.T) {
	assert.Equal(t, "CREATE", db.OperationTypeCreate.String())
	assert.Equal(t, "UPDATE", db.OperationTypeUpdate.String())
	assert.Equal(t, "DELETE", db.OperationTypeDelete.String())
	assert.Equal(t, "UNKNOWN", db.OperationType(99).String())
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

	op := &db.Operation{
		Steps: []db.OperationStep{
			{Name: stepCreateServiceAccount, Description: "Creating service account...", Completed: false},
			{Name: stepDeployInfrastructure, Description: "Deploying infrastructure...", Completed: false},
		},
	}

	service.completeStep(op, "test-server", stepCreateServiceAccount)

	assert.True(t, op.Steps[0].Completed)
	assert.False(t, op.Steps[1].Completed)
	assert.Equal(t, stepCreateServiceAccount, op.CurrentStep)
}

func TestCompleteStepNotFound(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	op := &db.Operation{
		Steps: []db.OperationStep{
			{Name: stepCreateServiceAccount, Description: "Creating service account...", Completed: false},
		},
	}

	service.completeStep(op, "test-server", "non_existent_step")

	assert.False(t, op.Steps[0].Completed)
	assert.Equal(t, "non_existent_step", op.CurrentStep)
}

func TestUpdateStep(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	// Should not panic
	service.updateStep(context.Background(), "test-server", stepDeployInfrastructure, "Deploying infrastructure...")
}

func TestHandleError(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	mockDB.On("UpdateOperation", mock.Anything, "test-server", mock.AnythingOfType("*db.Operation")).Return(nil)

	op := &db.Operation{
		Steps: []db.OperationStep{
			{Name: stepDeployInfrastructure, Description: "Deploying infrastructure...", Completed: false},
		},
	}

	testErr := errors.New("deployment failed")
	err := service.handleError(op, context.Background(), "test-server", stepDeployInfrastructure, testErr)

	assert.Equal(t, testErr, err)
	assert.Equal(t, db.OperationStateFailed, op.State)
	assert.Equal(t, stepDeployInfrastructure, op.CurrentStep)
	assert.Equal(t, "deployment failed", op.Error)
	assert.Equal(t, "deployment failed", op.Steps[0].Error)
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

func TestUpdateOperation(t *testing.T) {
	mockDB := new(MockDB)
	wm, _ := pulumi.NewWorkspaceManager(context.Background(), "test-project", "test-bucket")
	service := NewProvisioningService(wm, mockDB)

	mockDB.On("UpdateOperation", mock.Anything, "test-server", mock.AnythingOfType("*db.Operation")).Return(nil)

	op := &db.Operation{
		ID:    "test-server-123",
		Type:  db.OperationTypeCreate,
		State: db.OperationStateRunning,
	}

	service.updateOperation(context.Background(), "test-server", op)

	mockDB.AssertExpectations(t)
	assert.NotZero(t, op.UpdatedAt)
}
