package servers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/dbtypes"
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
