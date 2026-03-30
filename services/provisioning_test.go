package services

import (
	"context"
	"errors"
	"testing"
	"time"

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

type MockWorkspaceManager struct {
	mock.Mock
}

func (m *MockWorkspaceManager) UpsertStack(ctx context.Context, name string, program interface{}) (interface{}, error) {
	args := m.Called(ctx, name, program)
	return args.Get(0), args.Error(1)
}

func (m *MockWorkspaceManager) DestroyStack(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
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
	assert.Equal(t, db.OperationState("PENDING"), db.OperationStatePending)
	assert.Equal(t, db.OperationState("RUNNING"), db.OperationStateRunning)
	assert.Equal(t, db.OperationState("COMPLETED"), db.OperationStateCompleted)
	assert.Equal(t, db.OperationState("FAILED"), db.OperationStateFailed)
	assert.Equal(t, db.OperationState("CANCELLED"), db.OperationStateCancelled)
}

func TestOperationType(t *testing.T) {
	assert.Equal(t, db.OperationType("CREATE"), db.OperationTypeCreate)
	assert.Equal(t, db.OperationType("UPDATE"), db.OperationTypeUpdate)
	assert.Equal(t, db.OperationType("DELETE"), db.OperationTypeDelete)
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
