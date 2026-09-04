package services

import (
	"context"
	"testing"
	"time"

	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/dbtypes"
	"github.com/nbyl/metio/internal/pulumi/programs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGoroutineExecutor_SeedsInitialOutputs(t *testing.T) {
	executor := NewGoroutineExecutor(5 * time.Second)

	statusCh := make(chan *db.ProvisioningStatus, 1)
	err := executor.StartOperation(context.Background(), "srv1", db.ProvisioningOperationRestore,
		map[string]string{"backupId": "b1", "snapshotId": "snap-1"},
		func(_ context.Context, status *db.ProvisioningStatus) error {
			statusCh <- status
			return nil
		})
	require.NoError(t, err)

	got := <-statusCh
	assert.Equal(t, db.ProvisioningOperationRestore, got.Operation)
	require.NotNil(t, got.Outputs)
	assert.Equal(t, "b1", got.Outputs["backupId"])
	assert.Equal(t, "snap-1", got.Outputs["snapshotId"])
}

func newRestoreTestService() (*ProvisioningService, *MockDB, *[]*db.ProvisioningStatus) {
	svc, _, mockDB := newTestService()
	svc.SetBackupRestoreConfig("project-env-backups", "test-password")
	persisted := &[]*db.ProvisioningStatus{}
	mockDB.On("UpdateProvisioningStatus", mock.Anything, "srv1", mock.AnythingOfType("*db.ProvisioningStatus")).
		Run(func(args mock.Arguments) {
			*persisted = append(*persisted, args.Get(2).(*db.ProvisioningStatus))
		}).Return(nil)
	return svc, mockDB, persisted
}

func restoreTestBackup() *dbtypes.Backup {
	return &dbtypes.Backup{
		ID:               "backup-1",
		ServerID:         "srv1",
		SnapshotID:       "snap-abc123",
		RepositoryPrefix: "servers/srv1/restic/",
		Status:           dbtypes.BackupStatusCompleted,
		MinecraftVersion: "1.21.1",
	}
}

func TestRestoreServer_Success(t *testing.T) {
	svc, mockDB, persisted := newRestoreTestService()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{Name: "test", MinecraftVersion: "1.21.1"}, nil)
	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{
		ServerState:          dbtypes.ServerStateRunning,
		PendingCommandResult: "completed",
	}, nil)
	var savedCommands []string
	var savedArgs []map[string]string
	mockDB.On("UpdateStatus", mock.Anything, "test", mock.AnythingOfType("dbtypes.Status")).
		Run(func(args mock.Arguments) {
			st := args.Get(2).(dbtypes.Status)
			if st.PendingCommand != "" {
				savedCommands = append(savedCommands, st.PendingCommand)
				savedArgs = append(savedArgs, st.PendingCommandArgs)
			}
		}).Return(nil)

	err := svc.RestoreServer(context.Background(), "srv1", restoreTestBackup(), "")
	require.NoError(t, err)

	waitForPersisted(t, persisted, func(s *db.ProvisioningStatus) bool {
		return s.State == db.ProvisioningStateCompleted
	})

	assert.Equal(t, []string{"save", "restore"}, savedCommands)
	require.Len(t, savedArgs, 2)
	assert.Nil(t, savedArgs[0])
	assert.Equal(t, map[string]string{
		"snapshotId": "snap-abc123",
		"repository": "gs:project-env-backups:/servers/srv1/restic",
		"password":   "test-password",
	}, savedArgs[1])

	last := (*persisted)[len(*persisted)-1]
	assert.Equal(t, db.ProvisioningOperationRestore, last.Operation)
	assert.Equal(t, "snap-abc123", last.Outputs["snapshotId"])
	for _, step := range last.Steps {
		assert.Equal(t, db.ProvisioningStateCompleted, step.Status, "step %s should be completed", step.Name)
	}
}

func TestRestoreServer_VersionWarningPreservedInOutputs(t *testing.T) {
	svc, mockDB, persisted := newRestoreTestService()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{Name: "test", MinecraftVersion: "1.20.4"}, nil)
	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{
		ServerState:          dbtypes.ServerStateRunning,
		PendingCommandResult: "completed",
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test", mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := svc.RestoreServer(context.Background(), "srv1", restoreTestBackup(),
		"Backup was created with Minecraft 1.21.1 but server runs 1.20.4")
	require.NoError(t, err)

	waitForPersisted(t, persisted, func(s *db.ProvisioningStatus) bool {
		return s.State == db.ProvisioningStateCompleted
	})

	last := (*persisted)[len(*persisted)-1]
	assert.Equal(t, "Backup was created with Minecraft 1.21.1 but server runs 1.20.4",
		last.Outputs["versionMismatchWarning"])
}

func TestRestoreServer_WorldSaveFails(t *testing.T) {
	svc, mockDB, persisted := newRestoreTestService()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{Name: "test"}, nil)
	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{
		PendingCommandResult: "failed: rcon save-all failed",
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test", mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := svc.RestoreServer(context.Background(), "srv1", restoreTestBackup(), "")
	require.NoError(t, err) // async failure surfaces via provisioning status

	waitForPersisted(t, persisted, func(s *db.ProvisioningStatus) bool {
		return s.State == db.ProvisioningStateFailed
	})

	last := (*persisted)[len(*persisted)-1]
	assert.Contains(t, last.Error, "world save failed")
}

func TestRestoreServer_RestoreCommandFails(t *testing.T) {
	svc, mockDB, persisted := newRestoreTestService()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{Name: "test"}, nil)
	// Sequenced GetStatus responses: world-save trigger, world-save ack,
	// restore trigger, restore ack reporting agent-side rollback.
	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{ServerState: dbtypes.ServerStateRunning}, nil).Once()
	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{PendingCommandResult: "completed"}, nil).Once()
	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{ServerState: dbtypes.ServerStateStopped}, nil).Once()
	mockDB.On("GetStatus", mock.Anything, "test").
		Return(db.Status{PendingCommandResult: "failed: restic restore failed; rolled back to previous world"}, nil).
		Once()
	mockDB.On("UpdateStatus", mock.Anything, "test", mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := svc.RestoreServer(context.Background(), "srv1", restoreTestBackup(), "")
	require.NoError(t, err)

	waitForPersisted(t, persisted, func(s *db.ProvisioningStatus) bool {
		return s.State == db.ProvisioningStateFailed
	})

	last := (*persisted)[len(*persisted)-1]
	assert.Contains(t, last.Error, "restore failed")
}

func TestRestoreServer_RejectedWhileOperationInProgress(t *testing.T) {
	svc, _, _ := newRestoreTestService()

	blockCh := make(chan struct{})
	err := svc.executor.StartOperation(context.Background(), "srv1", db.ProvisioningOperationUpdate, nil,
		func(ctx context.Context, _ *db.ProvisioningStatus) error {
			<-blockCh
			return ctx.Err()
		})
	require.NoError(t, err)

	err = svc.RestoreServer(context.Background(), "srv1", restoreTestBackup(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation already in progress")

	close(blockCh)
}

func TestExecuteOperation_DispatchesRestore(t *testing.T) {
	svc, mockDB, _ := newRestoreTestService()

	now := time.Now()
	mockDB.On("GetProvisioningStatus", mock.Anything, "srv1").Return(&db.ProvisioningStatus{
		ID:        "srv1-123",
		Operation: db.ProvisioningOperationRestore,
		State:     db.ProvisioningStateInProgress,
		StartedAt: now,
		Outputs:   map[string]string{"snapshotId": "snap-abc123", "repositoryPrefix": "servers/srv1/restic/"},
	}, nil)
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{Name: "test"}, nil)
	mockDB.On("GetStatus", mock.Anything, "test").Return(db.Status{
		ServerState:          dbtypes.ServerStateRunning,
		PendingCommandResult: "completed",
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test", mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := svc.ExecuteOperation(context.Background(), "srv1", &programs.ServerConfig{Name: "test"}, updateTypeInPlace)
	require.NoError(t, err)
}

// waitForPersisted polls until one of the captured provisioning statuses satisfies
// the predicate or a timeout elapses.
func waitForPersisted(t *testing.T, persisted *[]*db.ProvisioningStatus, pred func(*db.ProvisioningStatus) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, s := range *persisted {
			if pred(s) {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for expected provisioning status")
}
