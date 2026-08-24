package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeBucketHandle struct {
	deleteResults map[string]struct {
		deleted int64
		err     error
	}
	calls []string
}

func (f *fakeBucketHandle) Attrs(ctx context.Context) (*storage.BucketAttrs, error) {
	return nil, nil
}

func (f *fakeBucketHandle) Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) error {
	return nil
}

func (f *fakeBucketHandle) DeletePrefix(ctx context.Context, prefix string) (int64, error) {
	f.calls = append(f.calls, prefix)
	result, ok := f.deleteResults[prefix]
	if !ok {
		return 0, nil
	}
	return result.deleted, result.err
}

type fakeCleanupStorageClient struct {
	bucket *fakeBucketHandle
}

func (f *fakeCleanupStorageClient) Bucket(name string) StorageBucketHandle {
	return f.bucket
}

func newCleanupTestSetup(deleteResults map[string]struct {
	deleted int64
	err     error
}) (*BackupCleanupService, *testutil.MockDB, *fakeBucketHandle) {
	bucket := &fakeBucketHandle{deleteResults: deleteResults}
	mockDB := new(testutil.MockDB)
	svc := NewBackupCleanupService(mockDB, &fakeCleanupStorageClient{bucket: bucket}, "proj-env-backups")
	return svc, mockDB, bucket
}

func expiredBackup(serverID, snapshotID string, retentionUntil time.Time) *db.Backup {
	until := retentionUntil
	deletedAt := until.AddDate(0, 0, -30)
	return &db.Backup{
		ID:               serverID + ":" + snapshotID,
		ServerID:         serverID,
		ServerName:       "srv",
		SnapshotID:       snapshotID,
		RepositoryPrefix: "servers/" + serverID + "/restic/",
		CreatedAt:        deletedAt.Add(-time.Hour),
		Status:           db.BackupStatusCompleted,
		ServerDeletedAt:  &deletedAt,
		RetentionUntil:   &until,
	}
}

func TestRunSweep_CleansExpiredServers(t *testing.T) {
	now := time.Now()
	expiredA := now.Add(-24 * time.Hour)
	expiredB := now.Add(-time.Hour)
	retained := now.Add(48 * time.Hour)

	svc, mockDB, bucket := newCleanupTestSetup(map[string]struct {
		deleted int64
		err     error
	}{
		"servers/srv-a/restic/": {deleted: 42, err: nil},
		"servers/srv-b/restic/": {deleted: 7, err: nil},
	})

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{
		expiredBackup("srv-a", "snap1", expiredA),
		expiredBackup("srv-b", "snap2", expiredB),
		func() *db.Backup {
			b := expiredBackup("srv-c", "snap3", retained)
			return b
		}(),
		{ServerID: "srv-live", SnapshotID: "snap4", Status: db.BackupStatusCompleted},
	}, nil)
	mockDB.On("DeleteServerBackups", mock.Anything, "srv-a").Return(nil)
	mockDB.On("DeleteServerBackups", mock.Anything, "srv-b").Return(nil)

	result, err := svc.RunSweep(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 3, result.ServersScanned)
	assert.Equal(t, 2, result.ServersCleaned)
	assert.Equal(t, int64(49), result.ObjectsDeleted)
	assert.Zero(t, result.ServersFailed)
	assert.ElementsMatch(t, []string{"servers/srv-a/restic/", "servers/srv-b/restic/"}, bucket.calls)
	mockDB.AssertCalled(t, "DeleteServerBackups", mock.Anything, "srv-a")
	mockDB.AssertCalled(t, "DeleteServerBackups", mock.Anything, "srv-b")
	mockDB.AssertNotCalled(t, "DeleteServerBackups", "srv-c")
}

func TestRunSweep_PartiallyRetainedServer_NotCleaned(t *testing.T) {
	now := time.Now()

	svc, mockDB, bucket := newCleanupTestSetup(nil)

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{
		expiredBackup("srv-mixed", "old", now.Add(-time.Hour)),
		expiredBackup("srv-mixed", "recent", now.Add(72*time.Hour)),
	}, nil)

	result, err := svc.RunSweep(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, result.ServersScanned)
	assert.Zero(t, result.ServersCleaned)
	assert.Empty(t, bucket.calls)
	mockDB.AssertNotCalled(t, "DeleteServerBackups", mock.Anything, mock.Anything)
}

func TestRunSweep_MissingPrefix_IsIdempotent(t *testing.T) {
	now := time.Now()

	svc, mockDB, _ := newCleanupTestSetup(nil)

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{
		expiredBackup("srv-gone", "snap1", now.Add(-time.Hour)),
	}, nil)
	mockDB.On("DeleteServerBackups", mock.Anything, "srv-gone").Return(nil)

	result, err := svc.RunSweep(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, result.ServersCleaned)
	assert.Zero(t, result.ObjectsDeleted)
	mockDB.AssertCalled(t, "DeleteServerBackups", mock.Anything, "srv-gone")
}

func TestRunSweep_PrefixDeletionFailure_RetainsCatalogAndContinues(t *testing.T) {
	now := time.Now()

	svc, mockDB, _ := newCleanupTestSetup(map[string]struct {
		deleted int64
		err     error
	}{
		"servers/srv-fail/restic/": {deleted: 0, err: errors.New("backend error")},
		"servers/srv-ok/restic/":   {deleted: 3, err: nil},
	})

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{
		expiredBackup("srv-fail", "snap1", now.Add(-time.Hour)),
		expiredBackup("srv-ok", "snap2", now.Add(-2*time.Hour)),
	}, nil)
	mockDB.On("DeleteServerBackups", mock.Anything, "srv-ok").Return(nil)

	result, err := svc.RunSweep(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, result.ServersCleaned)
	assert.Equal(t, 1, result.ServersFailed)
	assert.Equal(t, int64(3), result.ObjectsDeleted)
	mockDB.AssertNotCalled(t, "DeleteServerBackups", "srv-fail")
}

func TestRunSweep_CatalogRemovalFailure_FailsServerForRetry(t *testing.T) {
	now := time.Now()

	svc, mockDB, _ := newCleanupTestSetup(map[string]struct {
		deleted int64
		err     error
	}{
		"servers/srv-x/restic/": {deleted: 5, err: nil},
	})

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{
		expiredBackup("srv-x", "snap1", now.Add(-time.Hour)),
	}, nil)
	mockDB.On("DeleteServerBackups", mock.Anything, "srv-x").Return(errors.New("statestore unavailable"))

	result, err := svc.RunSweep(context.Background())

	assert.NoError(t, err)
	assert.Zero(t, result.ServersCleaned)
	assert.Equal(t, 1, result.ServersFailed)
	assert.Equal(t, int64(5), result.ObjectsDeleted)
}

func TestRunSweep_ListBackupsError_Propagates(t *testing.T) {
	svc, mockDB, _ := newCleanupTestSetup(nil)

	mockDB.On("ListBackups", mock.Anything).Return(nil, errors.New("statestore down"))

	result, err := svc.RunSweep(context.Background())

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestRunSweep_EmptyCatalog_NoOp(t *testing.T) {
	svc, mockDB, bucket := newCleanupTestSetup(nil)

	mockDB.On("ListBackups", mock.Anything).Return([]*db.Backup{}, nil)

	result, err := svc.RunSweep(context.Background())

	assert.NoError(t, err)
	assert.Zero(t, result.ServersScanned)
	assert.Zero(t, result.ServersCleaned)
	assert.Empty(t, bucket.calls)
}
