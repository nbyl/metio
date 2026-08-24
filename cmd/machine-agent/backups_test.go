package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nbyl/metio/internal/agentclient"
	"github.com/nbyl/metio/internal/backupmanifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// writeManifestFile stores a manifest under dir using the shared filename
// convention and returns its path.
func writeManifestFile(t *testing.T, dir string, m backupmanifest.Manifest) string {
	t.Helper()
	filename, err := backupmanifest.Save(dir, m)
	require.NoError(t, err)
	return filepath.Join(dir, filename)
}

func stubVersionFunc(t *testing.T) {
	t.Helper()
	orig := getMinecraftVersionFunc
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	t.Cleanup(func() { getMinecraftVersionFunc = orig })
}

func validManifest() backupmanifest.Manifest {
	return backupmanifest.Manifest{
		Timestamp:        "2026-08-17T10:30:00Z",
		SnapshotID:       "0123abcd",
		ServerID:         "srv-abcd1234",
		RepositoryPrefix: "servers/srv-abcd1234/restic/",
		DurationSeconds:  30,
		FileCount:        120,
		RepositorySize:   654321,
		Method:           "restic",
		Status:           backupmanifest.StatusCompleted,
	}
}

func TestProcessBackupManifests_Success(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MANIFEST_DIR", dir)
	stubVersionFunc(t)

	path := writeManifestFile(t, dir, validManifest())

	mockClient := new(MockAgentClient)
	mockClient.On("SubmitBackupReport", mock.Anything, "srv-abcd1234", mock.MatchedBy(func(r agentclient.BackupReport) bool {
		return r.SnapshotID == "0123abcd" &&
			r.RepositoryPrefix == "servers/srv-abcd1234/restic/" &&
			r.DurationSeconds == 30 &&
			r.FileCount == 120 &&
			r.RepositorySize == 654321 &&
			r.MinecraftVersion == "1.21.4" &&
			r.Status == backupmanifest.StatusCompleted
	})).Return(nil)

	require.NoError(t, processBackupManifests(context.Background(), mockClient))

	assert.NoFileExists(t, path, "acknowledged manifest must be deleted")
	mockClient.AssertExpectations(t)
}

func TestProcessBackupManifests_TransientFailureKeepsFileForRetry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MANIFEST_DIR", dir)
	stubVersionFunc(t)

	path := writeManifestFile(t, dir, validManifest())
	second := validManifest()
	second.Timestamp = "2026-08-18T11:00:00Z"
	second.SnapshotID = "deadbeef"
	path2 := writeManifestFile(t, dir, second)

	mockClient := new(MockAgentClient)
	// Controller outage: first manifest hits a 503...
	mockClient.On("SubmitBackupReport", mock.Anything, "srv-abcd1234", mock.MatchedBy(
		func(r agentclient.BackupReport) bool { return r.SnapshotID == "0123abcd" }),
	).Return(&agentclient.HTTPStatusError{StatusCode: 503}).Once()
	// ...and the second one a network error.
	mockClient.On("SubmitBackupReport", mock.Anything, "srv-abcd1234", mock.MatchedBy(
		func(r agentclient.BackupReport) bool { return r.SnapshotID == "deadbeef" }),
	).Return(errors.New("request failed: connection refused")).Once()

	require.NoError(t, processBackupManifests(context.Background(), mockClient))

	assert.FileExists(t, path, "manifest must survive controller outage")
	assert.FileExists(t, path2, "manifest must survive network failure")
	assert.NoFileExists(t, path+".rejected")
	assert.NoFileExists(t, path2+".rejected")
}

func TestProcessBackupManifests_RetriesAfterOutage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MANIFEST_DIR", dir)
	stubVersionFunc(t)

	path := writeManifestFile(t, dir, validManifest())

	mockClient := new(MockAgentClient)
	mockClient.On("SubmitBackupReport", mock.Anything, "srv-abcd1234", mock.Anything).
		Return(errors.New("connection refused")).Once()
	require.NoError(t, processBackupManifests(context.Background(), mockClient))
	assert.FileExists(t, path)

	// Next tick the controller is back; the same file gets submitted again.
	mockClient.On("SubmitBackupReport", mock.Anything, "srv-abcd1234", mock.Anything).
		Return(nil).Once()
	require.NoError(t, processBackupManifests(context.Background(), mockClient))
	assert.NoFileExists(t, path)
}

func TestProcessBackupManifests_PermanentRejectionQuarantines(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MANIFEST_DIR", dir)
	stubVersionFunc(t)

	path := writeManifestFile(t, dir, validManifest())

	mockClient := new(MockAgentClient)
	mockClient.On("SubmitBackupReport", mock.Anything, "srv-abcd1234", mock.Anything).
		Return(&agentclient.HTTPStatusError{StatusCode: 400, Body: "invalid repositoryPrefix"})

	require.NoError(t, processBackupManifests(context.Background(), mockClient))

	assert.NoFileExists(t, path)
	assert.FileExists(t, path+".rejected")
}

func TestProcessBackupManifests_UnparsableJSONQuarantined(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MANIFEST_DIR", dir)
	stubVersionFunc(t)

	path := filepath.Join(dir, "manifest-20260817T103000Z.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))

	var mockClient MockAgentClient // no expectations: nothing may be submitted

	require.NoError(t, processBackupManifests(context.Background(), &mockClient))
	mockClient.AssertNotCalled(t, "SubmitBackupReport", mock.Anything, mock.Anything, mock.Anything)

	assert.NoFileExists(t, path)
	assert.FileExists(t, path+".invalid")
}

func TestProcessBackupManifests_MissingFieldsQuarantined(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MANIFEST_DIR", dir)
	stubVersionFunc(t)

	// A legacy manifest from before the schema extension has no server_id.
	path := writeManifestFile(t, dir, backupmanifest.Manifest{
		Timestamp:  "2026-01-01T00:00:00Z",
		SnapshotID: "oldstyle",
		Method:     "restic",
	})

	var mockClient MockAgentClient

	require.NoError(t, processBackupManifests(context.Background(), &mockClient))
	mockClient.AssertNotCalled(t, "SubmitBackupReport", mock.Anything, mock.Anything, mock.Anything)

	assert.NoFileExists(t, path)
	assert.FileExists(t, path+".rejected")
}

func TestProcessBackupManifests_DuplicateSnapshotsBothAcked(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MANIFEST_DIR", dir)
	stubVersionFunc(t)

	// Two queued manifests for the same snapshot (e.g. written before an
	// earlier ack could delete its file). The controller deduplicates by
	// (serverID, snapshotID): the replay returns 200 and both files are
	// removed safely.
	first := writeManifestFile(t, dir, validManifest())
	dup := validManifest()
	dup.Timestamp = "2026-08-17T11:00:00Z"
	dupPath := writeManifestFile(t, dir, dup)

	calls := 0
	mockClient := new(MockAgentClient)
	mockClient.On("SubmitBackupReport", mock.Anything, "srv-abcd1234", mock.Anything).
		Run(func(args mock.Arguments) { calls++ }).Return(nil).Twice()

	require.NoError(t, processBackupManifests(context.Background(), mockClient))

	assert.Equal(t, 2, calls)
	assert.NoFileExists(t, first)
	assert.NoFileExists(t, dupPath)
}

func TestProcessBackupManifests_EmptyDirectory(t *testing.T) {
	t.Setenv("MANIFEST_DIR", t.TempDir())

	var mockClient MockAgentClient

	require.NoError(t, processBackupManifests(context.Background(), &mockClient))
	mockClient.AssertNotCalled(t, "SubmitBackupReport", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessBackupManifests_MissingDirectory(t *testing.T) {
	t.Setenv("MANIFEST_DIR", filepath.Join(t.TempDir(), "does-not-exist"))

	var mockClient MockAgentClient

	// A missing directory (fresh server before the first backup) is not an
	// error; there is simply nothing to do.
	require.NoError(t, processBackupManifests(context.Background(), &mockClient))
}

func TestProcessBackupManifests_VersionFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MANIFEST_DIR", dir)

	orig := getMinecraftVersionFunc
	getMinecraftVersionFunc = func() (string, string, error) { return "", "", fmt.Errorf("rcon down") }
	t.Cleanup(func() { getMinecraftVersionFunc = orig })

	path := writeManifestFile(t, dir, validManifest())

	mockClient := new(MockAgentClient)
	mockClient.On("SubmitBackupReport", mock.Anything, "srv-abcd1234", mock.MatchedBy(
		func(r agentclient.BackupReport) bool { return r.MinecraftVersion == "Unknown" }),
	).Return(nil)

	require.NoError(t, processBackupManifests(context.Background(), mockClient))
	assert.NoFileExists(t, path)
	mockClient.AssertExpectations(t)
}

func TestIsPermanentRejection(t *testing.T) {
	assert.True(t, isPermanentRejection(&agentclient.HTTPStatusError{StatusCode: 400}))
	assert.False(t, isPermanentRejection(&agentclient.HTTPStatusError{StatusCode: 401}))
	assert.False(t, isPermanentRejection(&agentclient.HTTPStatusError{StatusCode: 500}))
	assert.False(t, isPermanentRejection(errors.New("network down")))
	assert.False(t, isPermanentRejection(nil))
}
