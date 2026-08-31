package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nbyl/metio/internal/dbtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRunResticRestore_UsesExecIntoBackupContainer(t *testing.T) {
	oldExec := execCommandContext
	defer func() { execCommandContext = oldExec }()

	var gotArgs []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{name}, args...)
		return exec.Command("true")
	}

	err := runResticRestore("snap1", "gs:bkt:/servers/src/restic", "pw")
	require.NoError(t, err)

	assert.Equal(t, "/usr/bin/docker", gotArgs[0])
	assert.Contains(t, gotArgs, "exec")
	assert.Contains(t, gotArgs, "-e")
	assert.Contains(t, gotArgs, "RESTIC_REPOSITORY=gs:bkt:/servers/src/restic")
	assert.Contains(t, gotArgs, "RESTIC_PASSWORD=pw")
	assert.Contains(t, gotArgs, backupContainer)
	assert.Contains(t, gotArgs, "restore")
	assert.Contains(t, gotArgs, "snap1")
	assert.Contains(t, gotArgs, "--target")
	assert.Contains(t, gotArgs, "/data")
}

func TestRunResticRestore_Error(t *testing.T) {
	oldExec := execCommandContext
	defer func() { execCommandContext = oldExec }()

	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := runResticRestore("snap1", "gs:bkt:/servers/src/restic", "pw")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restic restore failed")
}

func TestStopStartServices_UseNsenterSystemctl(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	var stopArgs, startArgs []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "stop" {
				stopArgs = append([]string{name}, args...)
			}
			if a == "start" {
				startArgs = append([]string{name}, args...)
			}
		}
		return exec.Command("true")
	}

	require.NoError(t, stopMinecraftServices())
	require.NoError(t, startMinecraftServices())

	assert.Equal(t, "/usr/bin/nsenter", stopArgs[0], "must run through nsenter")
	assert.Equal(t, hostCommand, stopArgs[:len(hostCommand)], "must run through nsenter with the host PID namespace")
	assert.Contains(t, stopArgs, "stop")
	assert.Contains(t, stopArgs, minecraftServiceContainer)
	assert.Contains(t, stopArgs, backupContainer)

	assert.Equal(t, "/usr/bin/nsenter", startArgs[0], "must run through nsenter")
	assert.Contains(t, startArgs, "start")
	assert.Contains(t, startArgs, minecraftServiceContainer)
	assert.Contains(t, startArgs, backupContainer)
}

func TestRestoreMinecraftWorld_RestartsServicesAfterFailure(t *testing.T) {
	var started, stopped bool
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		for _, a := range args {
			switch a {
			case "stop":
				stopped = true
			case "start":
				started = true
			}
		}
		return exec.Command("true")
	}

	oldExecCtx := execCommandContext
	defer func() { execCommandContext = oldExecCtx }()
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := restoreMinecraftWorld("snap1", "gs:bkt:/srv/restic", "pw")
	require.Error(t, err)
	assert.True(t, stopped, "services must be stopped before restore")
	assert.True(t, started, "services must be restarted even on failure")
	assert.Contains(t, err.Error(), "restic restore failed")
}

func TestRestoreMinecraftWorld_Success(t *testing.T) {
	var cmdLog []string
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "stop" || a == "start" {
				cmdLog = append(cmdLog, a)
			}
		}
		return exec.Command("true")
	}

	oldExecCtx := execCommandContext
	defer func() { execCommandContext = oldExecCtx }()
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	require.Nil(t, restoreMinecraftWorld("snap1", "gs:bkt:/srv/restic", "pw"))
	assert.Equal(t, []string{"stop", "start"}, cmdLog)
}

func stubRestoreFunc(t *testing.T, calls *[]string, retErr error) {
	t.Helper()
	orig := restoreMinecraftWorldFunc
	restoreMinecraftWorldFunc = func(snapshotID, repository, password string) error {
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
		PendingCommand: "restore",
		PendingCommandArgs: map[string]string{
			"snapshotId": "snap9",
			"repository": "gs:bkt:/servers/src/restic",
			"password":   "pw",
		},
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

func TestHandlePendingCommand_RestoreMissingCredentials(t *testing.T) {
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

	assert.Empty(t, calls)
	assert.Contains(t, saved.PendingCommandResult, "missing repository or password")
}

func TestHandlePendingCommand_RestoreFailureReportsError(t *testing.T) {
	var calls []string
	stubRestoreFunc(t, &calls, assert.AnError)

	mockClient := new(MockAgentClient)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		PendingCommand: "restore",
		PendingCommandArgs: map[string]string{
			"snapshotId": "snap9",
			"repository": "gs:bkt:/servers/src/restic",
			"password":   "pw",
		},
	}, nil)
	var saved dbtypes.Status
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).
		Run(func(args mock.Arguments) { saved = args.Get(1).(dbtypes.Status) }).Return(nil)

	require.NoError(t, handlePendingCommand(context.Background(), mockClient, "srv-1"))

	assert.Equal(t, []string{"snap9"}, calls)
	assert.Contains(t, saved.PendingCommandResult, "failed")
	assert.Empty(t, saved.PendingCommand)
}

func TestRestoreTimeoutDefault(t *testing.T) {
	assert.Equal(t, 30*time.Minute, restoreTimeout)
}

func TestHostCommand(t *testing.T) {
	assert.Equal(t, []string{"/usr/bin/nsenter", "-t", "1", "-m", "-u", "-i", "-n"}, hostCommand)
	assert.NotContains(t, strings.Join(hostCommand, " "), "unexpected")
}
