package tasks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/pulumi/programs"
	"github.com/nbyl/metio/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBuildProgramConfig_PropagatesBackupImage(t *testing.T) {
	cfg := config.Config{
		Environment:          "development2",
		ProjectID:            "minecraftbyl",
		MachineAgentImage:    "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:58edad6",
		BackupImage:          "europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:58edad6",
		BackupResticPassword: "restic-secret",
	}
	sc := &db.ServerConfig{
		ID:               "server-1",
		Name:             "test-server",
		Region:           "europe-west3",
		Zone:             "europe-west3-b",
		MachineType:      "e2-small",
		MinecraftVersion: "1.20.1",
		DiskSizeGB:       10,
		ExistingAddress:  "test-server-addr",
		InfraVersion:     programs.CurrentInfraVersion,
	}

	programConfig := buildProgramConfig(sc, cfg, "token")

	require.NotNil(t, programConfig)
	assert.Equal(t, cfg.BackupImage, programConfig.BackupImage)
	assert.Equal(t, cfg.MachineAgentImage, programConfig.MachineAgentImage)
	assert.Equal(t, cfg.ProjectID, programConfig.GCPProject)
	assert.Equal(t, sc.Name, programConfig.Name)
	assert.Nil(t, programConfig.Backup)
}

func TestBuildProgramConfig_PropagatesPerServerBackupOverride(t *testing.T) {
	cfg := config.Config{
		Environment: "development2",
		ProjectID:   "minecraftbyl",
		BackupImage: "europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:58edad6",
	}
	sc := &db.ServerConfig{
		ID:               "server-1",
		Name:             "test-server",
		Region:           "europe-west3",
		Zone:             "europe-west3-b",
		MinecraftVersion: "1.20.1",
		Backup: &db.BackupConfig{
			Enabled:             true,
			BackupIntervalHours: 3,
			Keep:                14,
			KeepUnit:            "daily",
		},
	}

	programConfig := buildProgramConfig(sc, cfg, "token")

	require.NotNil(t, programConfig)
	require.NotNil(t, programConfig.Backup)
	assert.Equal(t, cfg.BackupImage, programConfig.BackupImage)
	assert.True(t, programConfig.Backup.Enabled)
	assert.Equal(t, 3, programConfig.Backup.BackupIntervalHours)
	assert.Equal(t, 14, programConfig.Backup.Keep)
	assert.Equal(t, "daily", programConfig.Backup.KeepUnit)
}

func TestHandleProvisioningTask_SkipsStaleOp(t *testing.T) {
	mockDB := new(testutil.MockDB)
	mockDB.On("GetProvisioningStatus", mock.Anything, "server-1").Return(
		&db.ProvisioningStatus{ID: "server-1-newer", State: db.ProvisioningStateInProgress}, nil)

	oldGetDB := getDBConnection
	oldExec := executeOperation
	defer func() {
		getDBConnection = oldGetDB
		executeOperation = oldExec
	}()
	getDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{}, nil
	}
	executeOperation = func(ctx context.Context, serverID string, programConfig *programs.ServerConfig, updateType int) error {
		t.Fatal("executeOperation must not be called for a stale task")
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/tasks/provision/server-1?opId=server-1-old", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "server-1"})
	rec := httptest.NewRecorder()

	HandleProvisioningTask(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockDB.AssertNotCalled(t, "GetServerConfig", mock.Anything, "server-1")
}

func TestHandleProvisioningTask_SkipsTerminalOp(t *testing.T) {
	mockDB := new(testutil.MockDB)
	mockDB.On("GetProvisioningStatus", mock.Anything, "server-1").Return(
		&db.ProvisioningStatus{ID: "server-1-op", State: db.ProvisioningStateCompleted}, nil)

	oldGetDB := getDBConnection
	oldExec := executeOperation
	defer func() {
		getDBConnection = oldGetDB
		executeOperation = oldExec
	}()
	getDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{}, nil
	}
	executeOperation = func(ctx context.Context, serverID string, programConfig *programs.ServerConfig, updateType int) error {
		t.Fatal("executeOperation must not be called for a terminal operation")
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/tasks/provision/server-1?opId=server-1-op", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "server-1"})
	rec := httptest.NewRecorder()

	HandleProvisioningTask(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockDB.AssertNotCalled(t, "GetServerConfig", mock.Anything, "server-1")
}

func TestHandleProvisioningTask_ProceedsForActiveOp(t *testing.T) {
	mockDB := new(testutil.MockDB)
	mockDB.On("GetProvisioningStatus", mock.Anything, "server-1").Return(
		&db.ProvisioningStatus{ID: "server-1-op", State: db.ProvisioningStateInProgress}, nil)
	mockDB.On("GetServerConfig", mock.Anything, "server-1").Return(
		&db.ServerConfig{
			ID:               "server-1",
			Name:             "test-server",
			Region:           "europe-west3",
			Zone:             "europe-west3-b",
			MachineType:      "e2-small",
			MinecraftVersion: "1.20.1",
			DiskSizeGB:       10,
		}, nil)

	oldGetDB := getDBConnection
	oldExec := executeOperation
	executed := false
	defer func() {
		getDBConnection = oldGetDB
		executeOperation = oldExec
	}()
	getDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{MachineAgentImage: "machine-agent-img", OperationMode: "cloudtasks"}, nil
	}
	executeOperation = func(ctx context.Context, serverID string, programConfig *programs.ServerConfig, updateType int) error {
		executed = true
		assert.Equal(t, "server-1", serverID)
		assert.Equal(t, "machine-agent-img", programConfig.MachineAgentImage)
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/tasks/provision/server-1?opId=server-1-op", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "server-1"})
	rec := httptest.NewRecorder()

	HandleProvisioningTask(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, executed)
}
