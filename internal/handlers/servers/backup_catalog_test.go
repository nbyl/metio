package servers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/dbtypes"
	"github.com/nbyl/metio/internal/handlers/agent"
	"github.com/nbyl/metio/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestListAllBackups_ReturnsGlobalCatalog(t *testing.T) {
	mockDB := new(testutil.MockDB)
	retention := time.Now().Add(24 * time.Hour)
	deletedAt := retention.AddDate(0, 0, -30)

	original := GetDBConnection
	GetDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{}, nil
	}
	defer func() { GetDBConnection = original }()

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{
		{
			ID:               "srv1:snap1",
			ServerID:         "srv1",
			ServerName:       "survival",
			SnapshotID:       "snap1",
			RepositoryPrefix: "servers/srv1/restic/",
			CreatedAt:        deletedAt.Add(-time.Hour),
			Status:           dbtypes.BackupStatusCompleted,
			ServerDeletedAt:  &deletedAt,
			RetentionUntil:   &retention,
			SourceConfig: &dbtypes.BackupSourceConfig{
				Region:           "europe-west6",
				Zone:             "europe-west6-a",
				MachineType:      "e2-small",
				DiskSizeGB:       20,
				MinecraftVersion: "1.21.1",
			},
		},
	}, nil)

	req := httptest.NewRequest("GET", "/api/backups", nil)
	w := httptest.NewRecorder()

	ListAllBackups(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp []backupResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, "srv1:snap1", resp[0].ID)
	assert.NotNil(t, resp[0].SourceConfig)
	assert.Equal(t, "europe-west6-a", resp[0].SourceConfig.Zone)
}

func TestListAllBackups_EmptyCatalogReturnsArray(t *testing.T) {
	mockDB := new(testutil.MockDB)

	original := GetDBConnection
	GetDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{}, nil
	}
	defer func() { GetDBConnection = original }()

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{}, nil)

	req := httptest.NewRequest("GET", "/api/backups", nil)
	w := httptest.NewRecorder()

	ListAllBackups(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]\n", w.Body.String())
}

func TestListAllBackups_DBErrorReturns500(t *testing.T) {
	mockDB := new(testutil.MockDB)

	original := GetDBConnection
	GetDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{}, nil
	}
	defer func() { GetDBConnection = original }()

	mockDB.On("ListBackups", mock.Anything).Return(nil, errors.New("statestore down"))

	req := httptest.NewRequest("GET", "/api/backups", nil)
	w := httptest.NewRecorder()

	ListAllBackups(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func setupCreateFromBackupTest(t *testing.T) (*testutil.MockDB, *testutil.MockProvisioningService, func()) {
	t.Helper()
	mockDB := new(testutil.MockDB)
	mockPS := new(testutil.MockProvisioningService)

	originalDB := GetDBConnection
	GetDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{MachineAgentImage: "agent:latest"}, nil
	}
	originalPS := ProvisioningService
	ProvisioningService = mockPS

	agent.SetSigningKey([]byte("test-secret"))

	return mockDB, mockPS, func() {
		GetDBConnection = originalDB
		ProvisioningService = originalPS
	}
}

func createFromBackupRequest(backupID string, body interface{}) *httptest.ResponseRecorder {
	jsonBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/backups/"+backupID+"/servers", bytes.NewReader(jsonBytes))
	req = mux.SetURLVars(req, map[string]string{"backupId": backupID})
	w := httptest.NewRecorder()
	CreateServerFromBackup(w, req)
	return w
}

func completedSourceBackup() *db.Backup {
	return &db.Backup{
		ID:               "old-srv:snap-abc",
		ServerID:         "old-srv",
		ServerName:       "old-server",
		SnapshotID:       "snap-abc",
		RepositoryPrefix: "servers/old-srv/restic",
		CreatedAt:        time.Now().Add(-time.Hour),
		Status:           dbtypes.BackupStatusCompleted,
		MinecraftVersion: "1.21.1",
		SourceConfig: &dbtypes.BackupSourceConfig{
			Region:           "europe-west6",
			Zone:             "europe-west6-a",
			MachineType:      "e2-small",
			DiskSizeGB:       20,
			MinecraftVersion: "1.21.1",
		},
	}
}

func TestCreateServerFromBackup_Success(t *testing.T) {
	mockDB, mockPS, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{completedSourceBackup()}, nil)
	mockDB.On("ListServerConfigs", mock.Anything).Return([]*db.ServerConfig{}, nil)
	mockDB.On("CreateServerConfig", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockPS.On("CreateServerFromBackup", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	w := createFromBackupRequest("old-srv:snap-abc", map[string]string{
		"name": "new-survival",
	})

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/api/servers/")
	assert.Contains(t, w.Header().Get("Location"), "/provisioning")

	var resp ServerResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "new-survival", resp.Config.Name)
	assert.Equal(t, "europe-west6", resp.Config.Region)
	assert.Equal(t, "europe-west6-a", resp.Config.Zone)
	assert.Equal(t, "e2-small", resp.Config.MachineType)
	assert.Equal(t, 20, resp.Config.DiskSizeGB)
	assert.Equal(t, "1.21.1", resp.Config.MinecraftVersion)

	// Verify the provisioning service was called with restore fields set.
	callArgs := mockPS.Calls[0].Arguments
	programConfig := callArgs.Get(2)
	assert.NotNil(t, programConfig)
}

func TestCreateServerFromBackup_OverridesApplied(t *testing.T) {
	mockDB, mockPS, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{completedSourceBackup()}, nil)
	mockDB.On("ListServerConfigs", mock.Anything).Return([]*db.ServerConfig{}, nil)
	mockDB.On("CreateServerConfig", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockPS.On("CreateServerFromBackup", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	w := createFromBackupRequest("old-srv:snap-abc", map[string]interface{}{
		"name":             "new-survival",
		"region":           "us-central1",
		"zone":             "us-central1-a",
		"machineType":      "e2-standard-4",
		"diskSizeGB":       50,
		"minecraftVersion": "1.20.4",
	})

	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp ServerResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "us-central1", resp.Config.Region)
	assert.Equal(t, "us-central1-a", resp.Config.Zone)
	assert.Equal(t, "e2-standard-4", resp.Config.MachineType)
	assert.Equal(t, 50, resp.Config.DiskSizeGB)
	assert.Equal(t, "1.20.4", resp.Config.MinecraftVersion)
}

func TestCreateServerFromBackup_BackupNotFound(t *testing.T) {
	mockDB, _, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{}, nil)

	w := createFromBackupRequest("nope:nope", map[string]string{
		"name": "new-server",
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateServerFromBackup_BackupNotCompleted(t *testing.T) {
	mockDB, _, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()

	failedBackup := completedSourceBackup()
	failedBackup.Status = dbtypes.BackupStatusFailed

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{failedBackup}, nil)

	w := createFromBackupRequest("old-srv:snap-abc", map[string]string{
		"name": "new-server",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateServerFromBackup_MissingName(t *testing.T) {
	mockDB, _, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{completedSourceBackup()}, nil)

	w := createFromBackupRequest("old-srv:snap-abc", map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateServerFromBackup_DuplicateName(t *testing.T) {
	mockDB, _, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{completedSourceBackup()}, nil)
	mockDB.On("ListServerConfigs", mock.Anything).Return([]*db.ServerConfig{
		{ID: "other-srv", Name: "existing-server"},
	}, nil)

	w := createFromBackupRequest("old-srv:snap-abc", map[string]string{
		"name": "existing-server",
	})

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateServerFromBackup_ProvisioningServiceUnavailable(t *testing.T) {
	mockDB, _, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()
	ProvisioningService = nil

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{completedSourceBackup()}, nil)
	mockDB.On("ListServerConfigs", mock.Anything).Return([]*db.ServerConfig{}, nil)
	mockDB.On("CreateServerConfig", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	w := createFromBackupRequest("old-srv:snap-abc", map[string]string{
		"name": "new-server",
	})

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCreateServerFromBackup_ProvisioningError(t *testing.T) {
	mockDB, mockPS, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{completedSourceBackup()}, nil)
	mockDB.On("ListServerConfigs", mock.Anything).Return([]*db.ServerConfig{}, nil)
	mockDB.On("CreateServerConfig", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockPS.On("CreateServerFromBackup", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("pulumi stack failed"))

	w := createFromBackupRequest("old-srv:snap-abc", map[string]string{
		"name": "new-server",
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateServerFromBackup_ConcurrentOperationConflict(t *testing.T) {
	mockDB, mockPS, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{completedSourceBackup()}, nil)
	mockDB.On("ListServerConfigs", mock.Anything).Return([]*db.ServerConfig{}, nil)
	mockDB.On("CreateServerConfig", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockPS.On("CreateServerFromBackup", mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("operation already in progress for server %s", "non-matching-id"))

	w := createFromBackupRequest("old-srv:snap-abc", map[string]string{
		"name": "new-server",
	})

	// Error message doesn't match the handler's serverID, so returns generic 500.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateServerFromBackup_DBErrorOnListBackups(t *testing.T) {
	mockDB, _, cleanup := setupCreateFromBackupTest(t)
	defer cleanup()

	mockDB.On("ListBackups", mock.Anything).Return(nil, errors.New("statestore down"))

	w := createFromBackupRequest("old-srv:snap-abc", map[string]string{
		"name": "new-server",
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
