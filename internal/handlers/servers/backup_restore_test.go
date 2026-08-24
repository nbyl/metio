package servers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/dbtypes"
	"github.com/nbyl/metio/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupRestoreHandlerTest(t *testing.T) (*testutil.MockDB, *testutil.MockProvisioningService, func()) {
	t.Helper()
	mockDB := new(testutil.MockDB)
	mockPS := new(testutil.MockProvisioningService)

	originalDB := GetDBConnection
	GetDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{}, nil
	}
	originalPS := ProvisioningService
	ProvisioningService = mockPS

	return mockDB, mockPS, func() {
		GetDBConnection = originalDB
		ProvisioningService = originalPS
	}
}

func performRestoreRequest(serverID, backupID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/servers/"+serverID+"/backups/"+backupID+"/restore", nil)
	req = mux.SetURLVars(req, map[string]string{"id": serverID, "backupId": backupID})
	w := httptest.NewRecorder()
	RestoreBackupByID(w, req)
	return w
}

func completedRestoreBackup() *db.Backup {
	return &db.Backup{
		ID:               "srv1:snap1",
		ServerID:         "srv1",
		ServerName:       "survival",
		SnapshotID:       "snap1",
		Status:           dbtypes.BackupStatusCompleted,
		MinecraftVersion: "1.21.1",
	}
}

func TestRestoreBackupByID_Success(t *testing.T) {
	mockDB, mockPS, cleanup := setupRestoreHandlerTest(t)
	defer cleanup()

	serverConfig := &db.ServerConfig{Name: "survival", MinecraftVersion: "1.21.1"}
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(serverConfig, nil)
	mockDB.On("GetBackup", mock.Anything, "srv1", "srv1:snap1").Return(completedRestoreBackup(), nil)
	mockPS.On("RestoreServer", mock.Anything, "srv1", mock.Anything, "").Return(nil)

	w := performRestoreRequest("srv1", "srv1:snap1")

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "/api/servers/srv1/provisioning", w.Header().Get("Location"))

	var resp RestoreResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "restore", resp.Operation)
	assert.Equal(t, "srv1", resp.ServerID)
	assert.Equal(t, "srv1:snap1", resp.BackupID)
	assert.Equal(t, "snap1", resp.SnapshotID)
	assert.Empty(t, resp.Warnings)
}

func TestRestoreBackupByID_VersionMismatchReturnsWarning(t *testing.T) {
	mockDB, mockPS, cleanup := setupRestoreHandlerTest(t)
	defer cleanup()

	serverConfig := &db.ServerConfig{Name: "survival", MinecraftVersion: "1.20.4"}
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(serverConfig, nil)
	mockDB.On("GetBackup", mock.Anything, "srv1", "srv1:snap1").Return(completedRestoreBackup(), nil)
	var capturedWarning string
	mockPS.On("RestoreServer", mock.Anything, "srv1", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			capturedWarning = args.String(3)
		}).Return(nil)

	w := performRestoreRequest("srv1", "srv1:snap1")

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, capturedWarning, "1.21.1")
	assert.Contains(t, capturedWarning, "1.20.4")

	var resp RestoreResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Warnings, 1)
}

func TestRestoreBackupByID_ServerNotFound(t *testing.T) {
	mockDB, _, cleanup := setupRestoreHandlerTest(t)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "missing").Return(nil, errors.New("not found"))

	w := performRestoreRequest("missing", "srv1:snap1")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRestoreBackupByID_BackupNotFound(t *testing.T) {
	mockDB, _, cleanup := setupRestoreHandlerTest(t)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{Name: "survival"}, nil)
	mockDB.On("GetBackup", mock.Anything, "srv1", "nope").Return(nil, errors.New("not found"))

	w := performRestoreRequest("srv1", "nope")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRestoreBackupByID_FailedBackupRejected(t *testing.T) {
	mockDB, _, cleanup := setupRestoreHandlerTest(t)
	defer cleanup()

	backup := completedRestoreBackup()
	backup.Status = dbtypes.BackupStatusFailed

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{Name: "survival"}, nil)
	mockDB.On("GetBackup", mock.Anything, "srv1", "srv1:snap1").Return(backup, nil)

	w := performRestoreRequest("srv1", "srv1:snap1")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRestoreBackupByID_ConcurrentOperationConflict(t *testing.T) {
	mockDB, mockPS, cleanup := setupRestoreHandlerTest(t)
	defer cleanup()

	serverConfig := &db.ServerConfig{Name: "survival", MinecraftVersion: "1.21.1"}
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(serverConfig, nil)
	mockDB.On("GetBackup", mock.Anything, "srv1", "srv1:snap1").Return(completedRestoreBackup(), nil)
	mockPS.On("RestoreServer", mock.Anything, "srv1", mock.Anything, "").
		Return(errors.New("operation already in progress for server srv1"))

	w := performRestoreRequest("srv1", "srv1:snap1")

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestBuildVersionMismatchWarning(t *testing.T) {
	assert.Empty(t, buildVersionMismatchWarning("", "1.21.1"))
	assert.Empty(t, buildVersionMismatchWarning("1.21.1", ""))
	assert.Empty(t, buildVersionMismatchWarning("1.21.1", "1.21.1"))
	assert.Contains(t, buildVersionMismatchWarning("1.21.1", "1.20.4"), "1.21.1")
	assert.Contains(t, buildVersionMismatchWarning("1.21.1", "1.20.4"), "1.20.4")
}
