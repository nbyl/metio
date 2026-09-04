package main

import (
	"context"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/nbyl/metio/internal/dbtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// captureTrue records the command invocation on the stub and returns a command
// that exits 0 with no output. echoOutput returns a command that emits the
// given value on stdout; they let the exec seams be stubbed per-call to test
// the parsed output of docker inspect.
func captureTrue(captured *[]string) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		*captured = append([]string{name}, args...)
		return exec.Command("true")
	}
}

func echoOutput(out string) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", out)
	}
}

func TestContainerImage(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()
	execCommandContext = echoOutput("europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:d8ca67f")

	img, err := containerImage(context.Background(), backupUnit)
	require.NoError(t, err)
	assert.Equal(t, "europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:d8ca67f", img)
}

func TestContainerImage_ErrorOnEmpty(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()
	execCommandContext = echoOutput("")

	_, err := containerImage(context.Background(), backupUnit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no image reference")
}

func TestContainerMountSource(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()
	execCommandContext = echoOutput("/mnt/disks/minecraft/data")

	src, err := containerMountSource(context.Background(), backupUnit, worldMountDest)
	require.NoError(t, err)
	assert.Equal(t, "/mnt/disks/minecraft/data", src)
}

func TestContainerMountSource_ErrorWhenMissing(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()
	execCommandContext = echoOutput("")

	_, err := containerMountSource(context.Background(), backupUnit, worldMountDest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no mount at destination")
}

func TestHostSystemctl_PrivilegedHelperInvocation(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()

	var got []string
	execCommandContext = captureTrue(&got)

	_, err := hostSystemctl(context.Background(), "agent-img:latest", "stop", minecraftUnit, backupUnit)
	require.NoError(t, err)

	assert.Equal(t, dockerBin, got[0], "must use the docker CLI")
	joined := got[1:]
	assert.Contains(t, joined, "run")
	assert.Contains(t, joined, "--rm")
	assert.Contains(t, joined, "--privileged")
	assert.Contains(t, joined, "--pid=host")
	assert.Contains(t, joined, "--user")
	assert.Contains(t, joined, "0")
	assert.Contains(t, joined, "--entrypoint")
	assert.Contains(t, joined, "/usr/bin/nsenter")
	assert.Contains(t, joined, "agent-img:latest")
	assert.Contains(t, joined, "-t")
	assert.Contains(t, joined, "1")
	assert.Contains(t, joined, "-m")
	assert.Contains(t, joined, "-u")
	assert.Contains(t, joined, "-i")
	assert.Contains(t, joined, "-n")
	assert.Contains(t, joined, hostSystemctlPath)
	assert.Contains(t, joined, "stop")
	assert.Contains(t, joined, minecraftUnit)
	assert.Contains(t, joined, backupUnit)
}

func TestStopMinecraftServices_StopsBothUnits(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()

	var got []string
	execCommandContext = captureTrue(&got)

	require.NoError(t, stopMinecraftServices(context.Background(), "agent-img:latest"))
	assert.Contains(t, got, "stop")
	assert.Contains(t, got, minecraftUnit)
	assert.Contains(t, got, backupUnit)
}

func TestStopMinecraftServices_Error(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := stopMinecraftServices(context.Background(), "agent-img:latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "systemctl stop failed")
}

func TestRunResticRestore_OneShotContainerInvocation(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()

	var got []string
	execCommandContext = captureTrue(&got)

	err := runResticRestore(context.Background(),
		"backup-img:latest", "/mnt/disks/minecraft/data", "snap1",
		"gs:bkt:/servers/src/restic", "pw")
	require.NoError(t, err)

	assert.Equal(t, dockerBin, got[0])
	joined := got[1:]
	assert.Contains(t, joined, "run")
	assert.Contains(t, joined, "--rm")
	assert.Contains(t, joined, "--network")
	assert.Contains(t, joined, "host")
	assert.Contains(t, joined, "-e")
	assert.Contains(t, joined, "RESTIC_REPOSITORY=gs:bkt:/servers/src/restic")
	assert.Contains(t, joined, "RESTIC_PASSWORD=pw")
	assert.Contains(t, joined, "-v")
	assert.Contains(t, joined, "/mnt/disks/minecraft/data:/data")
	assert.Contains(t, joined, "--entrypoint")
	assert.Contains(t, joined, "/usr/bin/restic")
	assert.Contains(t, joined, "backup-img:latest")
	assert.Contains(t, joined, "restore")
	assert.Contains(t, joined, "snap1:/data")
	assert.Contains(t, joined, "--target")
	i := slices.Index(joined, "--target")
	require.GreaterOrEqual(t, i, 0, "--target must be present")
	assert.Equal(t, "/data", joined[i+1])
	assert.NotContains(t, joined, "--delete")
}

func TestRunResticRestore_Error(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := runResticRestore(context.Background(),
		"backup-img:latest", "/mnt/disks/minecraft/data", "snap1",
		"gs:bkt:/servers/src/restic", "pw")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restic restore failed")
}

// stubInspectAndRestore installs exec seams that resolve container discovery to
// fixed values via echo-based commands, and delegates the actual restore/stop
// commands to a recorder. cmdLog receives one entry per command with its
// subcommand (inspect/stop/start/restore) for ordering assertions.
func stubInspectAndRestore(t *testing.T, cmdLog *[]string, restoreFails bool) {
	t.Helper()
	old := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		var joined []string
		argStr := ""
		if len(args) > 0 {
			joined = args
			argStr = joined[0]
		}
		switch argStr {
		case "inspect":
			// First inspect (agent image) returns the agent ref; the format
			// template determines which value to emit.
			if len(joined) > 2 && joined[2] == "{{.Config.Image}}" {
				return exec.Command("echo", "agent-img:latest")
			}
			return exec.Command("echo", "/mnt/disks/minecraft/data")
		case "run":
			*cmdLog = append(*cmdLog, "run")
			isRestic := false
			for _, a := range joined {
				if a == "/usr/bin/restic" {
					isRestic = true
					break
				}
			}
			if restoreFails && isRestic {
				return exec.Command("false")
			}
			return exec.Command("true")
		default:
			return exec.Command("true")
		}
	}
	t.Cleanup(func() { execCommandContext = old })
}

func TestRestoreMinecraftWorld_SuccessOrdering(t *testing.T) {
	var cmdLog []string
	stubInspectAndRestore(t, &cmdLog, false)

	require.NoError(t, restoreMinecraftWorld("snap1", "gs:bkt:/srv/restic", "pw"))
}

func TestRestoreMinecraftWorld_RestartsAfterFailure(t *testing.T) {
	var cmdLog []string
	stubInspectAndRestore(t, &cmdLog, true)

	err := restoreMinecraftWorld("snap1", "gs:bkt:/srv/restic", "pw")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restic restore failed")
}

func TestRestoreMinecraftWorld_ImageScanFailure(t *testing.T) {
	old := execCommandContext
	defer func() { execCommandContext = old }()
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := restoreMinecraftWorld("snap1", "gs:bkt:/srv/restic", "pw")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to identify agent image")
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
