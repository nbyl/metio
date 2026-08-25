package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyl/metio/internal/dbtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const backupUnitFixture = `#cloud-config fragment: rendered minecraft-backup.service
[Unit]
Description=Minecraft Backup Service
After=minecraft.service
Requires=minecraft.service

[Service]
User=metio
WorkingDirectory=/home/metio
TimeoutStartSec=0
Restart=always
ExecStartPre=-/usr/bin/docker stop %n
ExecStartPre=-/usr/bin/docker rm %n
ExecStart=/usr/bin/docker run --rm --name %n \
  --network host \
  -e BACKUP_METHOD=restic \
  -e BACKUP_INTERVAL=1h \
  -e RESTIC_REPOSITORY=gs:metio-dev-backups:/servers/srv-1234/restic \
  -e RESTIC_PASSWORD=super-secret \
  -v /mnt/disks/minecraft/data:/data \
  europe-docker.pkg.dev/metio/metio/mc-backup:latest
ExecStop=/usr/bin/docker stop %n

[Install]
WantedBy=default.target
`

func TestParseBackupUnitConfig(t *testing.T) {
	cfg, err := parseBackupUnitConfig(backupUnitFixture)
	require.NoError(t, err)
	assert.Equal(t, "gs:metio-dev-backups:/servers/srv-1234/restic", cfg.Repository)
	assert.Equal(t, "super-secret", cfg.Password)
	assert.Equal(t, "europe-docker.pkg.dev/metio/metio/mc-backup:latest", cfg.Image)
}

func TestParseBackupUnitConfig_MissingEnvironment(t *testing.T) {
	content := strings.ReplaceAll(backupUnitFixture, "-e RESTIC_PASSWORD=super-secret \\\n", "")
	_, err := parseBackupUnitConfig(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RESTIC_PASSWORD")
}

func TestPerformRestore_Success(t *testing.T) {
	base := t.TempDir()
	dataDir := filepath.Join(base, "data")
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "world"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "world", "level.dat"), []byte("old"), 0o644))
	staging := filepath.Join(base, stagingDirName)

	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "/usr/bin/docker" {
			script := fmt.Sprintf("mkdir -p %q/world && echo restored > %q/world/level.dat", staging, staging)
			return exec.Command("/bin/sh", "-c", script)
		}
		return exec.Command("true")
	}
	defer func() { execCommand = oldExec }()

	cfg := &backupUnitConfig{Repository: "gs:bkt:/servers/srv/restic", Password: "pw", Image: "backup-img"}
	require.NoError(t, performRestore(base, "snap1", cfg))

	restored, err := os.ReadFile(filepath.Join(dataDir, "world", "level.dat"))
	require.NoError(t, err)
	assert.Equal(t, "restored\n", string(restored))

	recoveryRoot := filepath.Join(base, recoveryDirName)
	entries, err := os.ReadDir(recoveryRoot)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	saved, err := os.ReadFile(filepath.Join(recoveryRoot, entries[0].Name(), "world", "level.dat"))
	require.NoError(t, err)
	assert.Equal(t, "old", string(saved), "previous world must be preserved in the recovery directory")

	assert.NoDirExists(t, staging, "staging directory must be cleaned up")
}

func TestPerformRestore_ResticFailureKeepsCurrentWorld(t *testing.T) {
	base := t.TempDir()
	dataDir := filepath.Join(base, "data")
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "world"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "world", "level.dat"), []byte("old"), 0o644))

	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "/usr/bin/docker" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	defer func() { execCommand = oldExec }()

	cfg := &backupUnitConfig{Repository: "gs:bkt:/servers/srv/restic", Password: "pw", Image: "backup-img"}
	err := performRestore(base, "snap1", cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restic restore failed")

	current, readErr := os.ReadFile(filepath.Join(dataDir, "world", "level.dat"))
	require.NoError(t, readErr, "current world must stay in place when the restore fails")
	assert.Equal(t, "old", string(current))
	assert.NoDirExists(t, filepath.Join(base, stagingDirName))
}

func TestRestoreMinecraftWorld_RestartsServicesAfterFailure(t *testing.T) {
	base := t.TempDir()
	dataDir := filepath.Join(base, "data")
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "world"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "world", "level.dat"), []byte("old"), 0o644))

	oldBase := minecraftBaseDir
	minecraftBaseDir = base
	defer func() { minecraftBaseDir = oldBase }()

	oldRead := osReadFile
	osReadFile = func(string) ([]byte, error) { return []byte(backupUnitFixture), nil }
	defer func() { osReadFile = oldRead }()

	var started bool
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "/usr/bin/docker" {
			return exec.Command("false")
		}
		started = true // systemctl start path after the rollback
		return exec.Command("true")
	}
	defer func() { execCommand = oldExec }()

	err := restoreMinecraftWorld("snap1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restic restore failed")

	current, readErr := os.ReadFile(filepath.Join(dataDir, "world", "level.dat"))
	require.NoError(t, readErr)
	assert.Equal(t, "old", string(current))
	assert.True(t, started, "Minecraft services must be restarted even on failure")
}

func stubRestoreFunc(t *testing.T, calls *[]string, retErr error) {
	t.Helper()
	orig := restoreMinecraftWorldFunc
	restoreMinecraftWorldFunc = func(snapshotID string) error {
		*calls = append(*calls, snapshotID)
		return retErr
	}
	t.Cleanup(func() { restoreMinecraftWorldFunc = orig })
}

func TestHandlePendingCommand_RestoreSuccess(t *testing.T) {
	var calls []string
	stubRestoreFunc(t, &calls, nil)

	mockClient := new(MockAgentClient)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		PendingCommand:     "restore",
		PendingCommandArgs: map[string]string{"snapshotId": "snap9"},
	}, nil)

	var saved dbtypes.Status
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).
		Run(func(args mock.Arguments) { saved = args.Get(1).(dbtypes.Status) }).Return(nil)

	require.NoError(t, handlePendingCommand(context.Background(), mockClient, "srv-1"))

	assert.Equal(t, []string{"snap9"}, calls)
	assert.Empty(t, saved.PendingCommand)
	assert.Nil(t, saved.PendingCommandArgs)
	assert.Equal(t, "completed", saved.PendingCommandResult)
}

func TestHandlePendingCommand_RestoreMissingSnapshotID(t *testing.T) {
	var calls []string
	stubRestoreFunc(t, &calls, nil)

	mockClient := new(MockAgentClient)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		PendingCommand: "restore",
	}, nil)
	var saved dbtypes.Status
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).
		Run(func(args mock.Arguments) { saved = args.Get(1).(dbtypes.Status) }).Return(nil)

	require.NoError(t, handlePendingCommand(context.Background(), mockClient, "srv-1"))

	assert.Empty(t, calls)
	assert.Contains(t, saved.PendingCommandResult, "missing snapshotId")
}

func TestHandlePendingCommand_RestoreFailureReportsRollbackError(t *testing.T) {
	var calls []string
	stubRestoreFunc(t, &calls, fmt.Errorf("restic restore failed; rolled back to previous world"))

	mockClient := new(MockAgentClient)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		PendingCommand:     "restore",
		PendingCommandArgs: map[string]string{"snapshotId": "snap9"},
	}, nil)
	var saved dbtypes.Status
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).
		Run(func(args mock.Arguments) { saved = args.Get(1).(dbtypes.Status) }).Return(nil)

	require.NoError(t, handlePendingCommand(context.Background(), mockClient, "srv-1"))

	assert.Equal(t, []string{"snap9"}, calls)
	assert.Contains(t, saved.PendingCommandResult, "rolled back to previous world")
	assert.Empty(t, saved.PendingCommand)
}
