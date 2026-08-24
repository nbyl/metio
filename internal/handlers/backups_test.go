package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nbyl/metio/internal/services"
	"github.com/stretchr/testify/assert"
)

type fakeCleanupSweeper struct {
	result *services.CleanupResult
	err    error
	calls  int
}

func (f *fakeCleanupSweeper) RunSweep(ctx context.Context) (*services.CleanupResult, error) {
	f.calls++
	return f.result, f.err
}

func TestHandleBackupCleanup_RequiresBearerAuth(t *testing.T) {
	sweeper := &fakeCleanupSweeper{}
	originalFactory := newBackupCleanupSweeper
	newBackupCleanupSweeper = func(ctx context.Context) (BackupCleanupSweeper, error) {
		return sweeper, nil
	}
	defer func() { newBackupCleanupSweeper = originalFactory }()

	req := httptest.NewRequest("POST", "/api/backups/cleanup", nil)
	w := httptest.NewRecorder()

	HandleBackupCleanup(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Zero(t, sweeper.calls)
}

func TestHandleBackupCleanup_ReturnsSweepResult(t *testing.T) {
	sweeper := &fakeCleanupSweeper{result: &services.CleanupResult{
		ServersScanned: 2,
		ServersCleaned: 1,
		ServersFailed:  1,
		ObjectsDeleted: 42,
	}}
	originalFactory := newBackupCleanupSweeper
	newBackupCleanupSweeper = func(ctx context.Context) (BackupCleanupSweeper, error) {
		return sweeper, nil
	}
	defer func() { newBackupCleanupSweeper = originalFactory }()

	req := httptest.NewRequest("POST", "/api/backups/cleanup", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()

	HandleBackupCleanup(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, sweeper.calls)

	var result services.CleanupResult
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, 2, result.ServersScanned)
	assert.Equal(t, 1, result.ServersCleaned)
	assert.Equal(t, 1, result.ServersFailed)
	assert.Equal(t, int64(42), result.ObjectsDeleted)
}

func TestHandleBackupCleanup_UnavailableDeploymentMode(t *testing.T) {
	originalFactory := newBackupCleanupSweeper
	newBackupCleanupSweeper = func(ctx context.Context) (BackupCleanupSweeper, error) {
		return nil, ErrBackupCleanupUnavailable
	}
	defer func() { newBackupCleanupSweeper = originalFactory }()

	req := httptest.NewRequest("POST", "/api/backups/cleanup", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()

	HandleBackupCleanup(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestHandleBackupCleanup_SweepFailureReturns500(t *testing.T) {
	sweeper := &fakeCleanupSweeper{err: errors.New("statestore down")}
	originalFactory := newBackupCleanupSweeper
	newBackupCleanupSweeper = func(ctx context.Context) (BackupCleanupSweeper, error) {
		return sweeper, nil
	}
	defer func() { newBackupCleanupSweeper = originalFactory }()

	req := httptest.NewRequest("POST", "/api/backups/cleanup", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()

	HandleBackupCleanup(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleBackupCleanup_FactoryFailureReturns500(t *testing.T) {
	originalFactory := newBackupCleanupSweeper
	newBackupCleanupSweeper = func(ctx context.Context) (BackupCleanupSweeper, error) {
		return nil, errors.New("storage client failed")
	}
	defer func() { newBackupCleanupSweeper = originalFactory }()

	req := httptest.NewRequest("POST", "/api/backups/cleanup", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()

	HandleBackupCleanup(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
