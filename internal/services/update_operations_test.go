package services

import (
	"context"
	"testing"
	"time"

	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBackupCoordinator_TriggerWorldSave(t *testing.T) {
	mockDB := new(testutil.MockDB)
	bc := NewBackupCoordinator(mockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.MatchedBy(func(s db.Status) bool {
		return s.PendingCommand == "save" && s.PendingCommandResult == ""
	})).Return(nil)

	err := bc.TriggerWorldSave(context.Background(), "test-instance")
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestBackupCoordinator_TriggerWorldSave_GetStatusError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	bc := NewBackupCoordinator(mockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, assert.AnError)

	err := bc.TriggerWorldSave(context.Background(), "test-instance")
	assert.Error(t, err)
}

func TestBackupCoordinator_TriggerWorldSave_NoStatusDocument(t *testing.T) {
	mockDB := new(testutil.MockDB)
	bc := NewBackupCoordinator(mockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, db.ErrNotFound)

	err := bc.TriggerWorldSave(context.Background(), "test-instance")
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestBackupCoordinator_TriggerWorldSave_UpdateStatusError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	bc := NewBackupCoordinator(mockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("dbtypes.Status")).Return(assert.AnError)

	err := bc.TriggerWorldSave(context.Background(), "test-instance")
	assert.Error(t, err)
}

func TestBackupCoordinator_WaitForCommandAck_Completed(t *testing.T) {
	mockDB := new(testutil.MockDB)
	bc := NewBackupCoordinator(mockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{PendingCommandResult: "completed"}, nil).Once()

	result, err := bc.WaitForCommandAck(context.Background(), "test-instance", 5*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, "completed", result)
}

func TestBackupCoordinator_WaitForCommandAck_ErrorResult(t *testing.T) {
	mockDB := new(testutil.MockDB)
	bc := NewBackupCoordinator(mockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{PendingCommandResult: "failed"}, nil).Once()

	result, err := bc.WaitForCommandAck(context.Background(), "test-instance", 5*time.Second)
	assert.Error(t, err)
	assert.Equal(t, "failed", result)
}

func TestBackupCoordinator_WaitForCommandAck_GetStatusError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	bc := NewBackupCoordinator(mockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, assert.AnError).Once()

	_, err := bc.WaitForCommandAck(context.Background(), "test-instance", 5*time.Second)
	assert.Error(t, err)
}

func TestBackupCoordinator_WaitForCommandAck_NoStatusDocument(t *testing.T) {
	mockDB := new(testutil.MockDB)
	bc := NewBackupCoordinator(mockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, db.ErrNotFound).Once()

	result, err := bc.WaitForCommandAck(context.Background(), "test-instance", 5*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, "", result)
	mockDB.AssertExpectations(t)
}

func TestBackupCoordinator_WaitForCommandAck_Timeout(t *testing.T) {
	mockDB := new(testutil.MockDB)
	bc := NewBackupCoordinator(mockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)

	_, err := bc.WaitForCommandAck(context.Background(), "test-instance", 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestWaitForServerHealthy_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)

	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{ServerState: "RUNNING"}, nil).Once()

	err := WaitForServerHealthy(context.Background(), mockDB, "test", 5*time.Second)
	assert.NoError(t, err)
}

func TestWaitForServerHealthy_GetStatusError(t *testing.T) {
	mockDB := new(testutil.MockDB)

	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{}, assert.AnError).Once()

	err := WaitForServerHealthy(context.Background(), mockDB, "test", 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestWaitForServerHealthy_Timeout(t *testing.T) {
	mockDB := new(testutil.MockDB)

	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{ServerState: "STARTING"}, nil)

	err := WaitForServerHealthy(context.Background(), mockDB, "test", 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}
