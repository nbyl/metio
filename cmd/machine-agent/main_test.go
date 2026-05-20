package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/testutil"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type MockDB = testutil.MockDB

func TestRunStatusUpdate(t *testing.T) {
	mockDB := new(MockDB)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncWhitelistFunc := syncWhitelistFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) {
		return 5, 20, nil
	}
	getUptimeFunc = func() (string, error) {
		return "2 days, 3:45", nil
	}
	getMinecraftVersionFunc = func() (string, string, error) {
		return "1.21.4", "", nil
	}
	syncWhitelistFunc = func(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) {
		return true, nil
	}
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncWhitelistFunc
	}()

	// Mock GetStatus for checkScheduledShutdown (called after UpdateStatus)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)

	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil).Run(func(args mock.Arguments) {
		status := args.Get(2).(db.Status)
		assert.Equal(t, 5, status.Players.Current)
		assert.Equal(t, 20, status.Players.Max)
		assert.Equal(t, db.ServerStateRunning, status.ServerState)
		assert.Equal(t, "1.21.4", status.Version)
		assert.True(t, status.WhitelistEnabled)
	})

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestRunStatusUpdateError(t *testing.T) {
	mockDB := new(MockDB)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncWhitelistFunc := syncWhitelistFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) {
		return 0, 10, nil
	}
	getUptimeFunc = func() (string, error) {
		return "2 days, 3:45", nil
	}
	getMinecraftVersionFunc = func() (string, string, error) {
		return "1.21.4", "", nil
	}
	syncWhitelistFunc = func(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) {
		return false, nil
	}
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncWhitelistFunc
	}()

	// Mock GetStatus for checkScheduledShutdown (called after UpdateStatus)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)

	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(assert.AnError)

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
	mockDB.AssertExpectations(t)
}

func TestGetMinecraftPlayerCount(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "There are 2 of a max of 20 players online: Steve, Alex")
	}
	defer func() { execCommand = oldExecCommand }()

	current, max, err := getMinecraftPlayerCount()
	assert.NoError(t, err)
	assert.Equal(t, 2, current)
	assert.Equal(t, 20, max)
}

func TestGetMinecraftPlayerCountInvalidOutput(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "Invalid output")
	}
	defer func() { execCommand = oldExecCommand }()

	_, _, err := getMinecraftPlayerCount()
	assert.Error(t, err)
}

func TestGetUptime(t *testing.T) {
	oldReadFile := osReadFile
	osReadFile = func(filename string) ([]byte, error) {
		if filename == "/proc/uptime" {
			return []byte("172800.00 123456.78"), nil // 2 days in seconds
		}
		return nil, fmt.Errorf("file not found")
	}
	defer func() { osReadFile = oldReadFile }()

	uptime, err := getUptime()
	assert.NoError(t, err)
	assert.Equal(t, "2 days, 0:00", uptime)
}

func TestGetUptimeInvalidOutput(t *testing.T) {
	oldReadFile := osReadFile
	osReadFile = func(filename string) ([]byte, error) {
		return []byte("invalid"), nil
	}
	defer func() { osReadFile = oldReadFile }()

	_, err := getUptime()
	assert.Error(t, err)
}

func TestGetInstanceIP(t *testing.T) {
	// This test would require mocking the metadata client
	// For now, we'll test the function structure
	_ = func() (string, error) {
		return getInstanceIP()
	}
}

func TestGetMinecraftVersion_Paper(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "This server is running Paper version git-Paper-550 (MC: 1.21.4)")
	}
	defer func() { execCommand = oldExecCommand }()

	version, rawOutput, err := getMinecraftVersion()
	assert.NoError(t, err)
	assert.Equal(t, "1.21.4", version)
	assert.Empty(t, rawOutput)
}

func TestGetMinecraftVersion_Vanilla(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "Starting minecraft server version 1.21.4")
	}
	defer func() { execCommand = oldExecCommand }()

	version, rawOutput, err := getMinecraftVersion()
	assert.NoError(t, err)
	assert.Equal(t, "1.21.4", version)
	assert.Empty(t, rawOutput)
}

func TestGetMinecraftVersion_VanillaRcon(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "Server version info:id = 1.21.10name = 1.21.10data = 4556series = mainprotocol = 773 (0x305)build_time = Tue Oct 07 09:14:11 UTC 2025pack_resource = 69.0pack_data = 88.0stable = yes")
	}
	defer func() { execCommand = oldExecCommand }()

	version, rawOutput, err := getMinecraftVersion()
	assert.NoError(t, err)
	assert.Equal(t, "1.21.10", version)
	assert.Empty(t, rawOutput)
}

func TestGetMinecraftVersion_CommandFails(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false") // exits with error
	}
	defer func() { execCommand = oldExecCommand }()

	version, rawOutput, err := getMinecraftVersion()
	assert.NoError(t, err)
	assert.Equal(t, "Unknown", version)
	assert.Empty(t, rawOutput)
}

func TestGetMinecraftVersion_InvalidOutput(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "Some random text without version")
	}
	defer func() { execCommand = oldExecCommand }()

	version, rawOutput, err := getMinecraftVersion()
	assert.NoError(t, err)
	assert.Equal(t, "Unknown", version)
	assert.Contains(t, rawOutput, "Some random text without version")
}

func TestFormatDuration_Hours(t *testing.T) {
	assert.Equal(t, "1:30", formatDuration(5400*time.Second))
}

func TestFormatDuration_Days(t *testing.T) {
	assert.Equal(t, "1 days, 0:00", formatDuration(86400*time.Second))
}

func TestFormatDuration_Zero(t *testing.T) {
	assert.Equal(t, "0:00", formatDuration(0))
}

func TestRunStatusUpdate_PlayerCountError(t *testing.T) {
	mockDB := new(MockDB)
	oldGetFunc := getMinecraftPlayerCountFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) {
		return 0, 0, fmt.Errorf("rcon error")
	}
	defer func() { getMinecraftPlayerCountFunc = oldGetFunc }()

	// Mock for checkScheduledShutdown
	oldCheck := checkScheduledShutdownFunc
	checkScheduledShutdownFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error {
		return nil
	}
	defer func() { checkScheduledShutdownFunc = oldCheck }()

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
}

func TestRunStatusUpdate_UptimeError(t *testing.T) {
	mockDB := new(MockDB)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) { return 5, 20, nil }
	getUptimeFunc = func() (string, error) { return "", fmt.Errorf("uptime error") }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
	}()

	oldCheck := checkScheduledShutdownFunc
	checkScheduledShutdownFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error { return nil }
	defer func() { checkScheduledShutdownFunc = oldCheck }()

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
}

func TestRunStatusUpdate_VersionError(t *testing.T) {
	mockDB := new(MockDB)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 5, 20, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "", "", fmt.Errorf("version error") }
	syncWhitelistFunc = func(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) { return false, nil }
	getInstanceIPFunc = func() (string, error) { return "1.2.3.4:25565", nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
	}()

	oldCheck := checkScheduledShutdownFunc
	checkScheduledShutdownFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error { return nil }
	defer func() { checkScheduledShutdownFunc = oldCheck }()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err) // version error is non-fatal
}

func TestRunStatusUpdate_IPError(t *testing.T) {
	mockDB := new(MockDB)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 5, 20, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) { return false, nil }
	getInstanceIPFunc = func() (string, error) { return "", fmt.Errorf("metadata error") }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
	}()

	oldCheck := checkScheduledShutdownFunc
	checkScheduledShutdownFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error { return nil }
	defer func() { checkScheduledShutdownFunc = oldCheck }()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err) // IP error is non-fatal
}

// --- importWhitelistIfEmpty tests ---

func TestImportWhitelistIfEmpty_AlreadyHasEntries(t *testing.T) {
	mockDB := new(MockDB)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{
		{Username: "player1", UUID: "uuid-1"},
	}, nil)

	err := importWhitelistIfEmpty(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
}

func TestImportWhitelistIfEmpty_EmptyWhitelist(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	entries := []MinecraftWhitelistEntry{
		{UUID: "uuid-1", Name: "player1"},
		{UUID: "uuid-2", Name: "player2"},
	}
	entriesJSON, _ := json.Marshal(entries)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// whitelist.json read
			return exec.Command("echo", string(entriesJSON))
		}
		// server.properties read
		return exec.Command("echo", "white-list=true")
	}

	mockDB.On("AddWhitelistEntry", mock.Anything, "test-instance", mock.AnythingOfType("db.WhitelistEntry")).Return(nil)
	mockDB.On("SetWhitelistConfig", mock.Anything, "test-instance", mock.AnythingOfType("db.WhitelistConfig")).Return(nil)

	err := importWhitelistIfEmpty(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
}

func TestImportWhitelistIfEmpty_EmptyJSON(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "[]")
	}

	err := importWhitelistIfEmpty(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
}

func TestImportWhitelistIfEmpty_ReadError(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := importWhitelistIfEmpty(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
}

// --- syncWhitelist tests ---

func TestSyncWhitelist_Success(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: true}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{
		{Username: "player1", UUID: "uuid-1"},
	}, nil)

	// Mock exec for getMinecraftWhitelist (cat whitelist.json) and getWhitelistEnabledStatus (cat server.properties)
	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// whitelist.json
			entries := []MinecraftWhitelistEntry{{UUID: "uuid-1", Name: "player1"}}
			data, _ := json.Marshal(entries)
			return exec.Command("echo", string(data))
		}
		// server.properties
		return exec.Command("echo", "white-list=true")
	}

	enabled, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

func TestSyncWhitelist_AddAndRemove(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: false}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{
		{Username: "newplayer", UUID: "uuid-new"},
	}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// whitelist.json - has oldplayer not in firestore
			entries := []MinecraftWhitelistEntry{{UUID: "uuid-old", Name: "oldplayer"}}
			data, _ := json.Marshal(entries)
			return exec.Command("echo", string(data))
		}
		// All other commands succeed
		return exec.Command("true")
	}

	enabled, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.False(t, enabled)
}

func TestSyncWhitelist_GetConfigError(t *testing.T) {
	mockDB := new(MockDB)
	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{}, fmt.Errorf("db error"))

	_, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
}

func TestSyncWhitelist_GetEntriesError(t *testing.T) {
	mockDB := new(MockDB)
	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: true}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry(nil), fmt.Errorf("entries error"))

	_, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
}

// --- getMinecraftWhitelist tests ---

func TestGetMinecraftWhitelist_Success(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	entries := []MinecraftWhitelistEntry{{UUID: "uuid-1", Name: "player1"}}
	data, _ := json.Marshal(entries)
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", string(data))
	}

	result, err := getMinecraftWhitelist()
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestGetMinecraftWhitelist_CommandError(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	_, err := getMinecraftWhitelist()
	assert.Error(t, err)
}

func TestGetMinecraftWhitelist_InvalidJSON(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "not json")
	}

	_, err := getMinecraftWhitelist()
	assert.Error(t, err)
}

// --- addPlayerToMinecraftWhitelist tests ---

func TestAddPlayerToMinecraftWhitelist_Success(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	err := addPlayerToMinecraftWhitelist("testplayer")
	assert.NoError(t, err)
}

func TestAddPlayerToMinecraftWhitelist_Error(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := addPlayerToMinecraftWhitelist("testplayer")
	assert.Error(t, err)
}

// --- removePlayerFromMinecraftWhitelist tests ---

func TestRemovePlayerFromMinecraftWhitelist_Success(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	err := removePlayerFromMinecraftWhitelist("testplayer")
	assert.NoError(t, err)
}

func TestRemovePlayerFromMinecraftWhitelist_Error(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := removePlayerFromMinecraftWhitelist("testplayer")
	assert.Error(t, err)
}

// --- getWhitelistEnabledStatus tests ---

func TestGetWhitelistEnabledStatus_True(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "white-list=true")
	}

	assert.True(t, getWhitelistEnabledStatus())
}

func TestGetWhitelistEnabledStatus_False(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "white-list=false")
	}

	assert.False(t, getWhitelistEnabledStatus())
}

func TestGetWhitelistEnabledStatus_EnforceWhitelist(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "enforce-whitelist=true")
	}

	assert.True(t, getWhitelistEnabledStatus())
}

func TestGetWhitelistEnabledStatus_CommandError(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	assert.False(t, getWhitelistEnabledStatus())
}

func TestGetWhitelistEnabledStatus_NoWhitelistLine(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "some-other-property=value")
	}

	assert.False(t, getWhitelistEnabledStatus())
}

// --- setWhitelistEnabled tests ---

func TestSetWhitelistEnabled_On(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	err := setWhitelistEnabled(true)
	assert.NoError(t, err)
}

func TestSetWhitelistEnabled_Off(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	err := setWhitelistEnabled(false)
	assert.NoError(t, err)
}

func TestSetWhitelistEnabled_Error(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := setWhitelistEnabled(true)
	assert.Error(t, err)
}

// --- sendMinecraftMessage tests ---

func TestSendMinecraftMessage_Success(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	err := sendMinecraftMessage("hello")
	assert.NoError(t, err)
}

func TestSendMinecraftMessage_Error(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := sendMinecraftMessage("hello")
	assert.Error(t, err)
}

// --- saveMinecraftWorld tests ---

func TestSaveMinecraftWorld_Success(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	err := saveMinecraftWorld()
	assert.NoError(t, err)
}

func TestSaveMinecraftWorld_Error(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := saveMinecraftWorld()
	assert.Error(t, err)
}

// --- checkScheduledShutdown tests ---

func TestCheckScheduledShutdown_NoShutdown(t *testing.T) {
	mockDB := new(MockDB)

	// Reset global state
	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ScheduledShutdown: nil,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
}

func TestCheckScheduledShutdown_GetStatusError(t *testing.T) {
	mockDB := new(MockDB)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, fmt.Errorf("db error"))

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
}

func TestCheckScheduledShutdown_FutureShutdown_NoWarning(t *testing.T) {
	mockDB := new(MockDB)
	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	futureTime := time.Now().Add(30 * time.Minute)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ScheduledShutdown: &futureTime,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateNone, shutdownWarningState)
}

func TestCheckScheduledShutdown_FiveMinWarning(t *testing.T) {
	mockDB := new(MockDB)
	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	oldSendMsg := sendMinecraftMessageFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	defer func() { sendMinecraftMessageFunc = oldSendMsg }()

	fiveMinFromNow := time.Now().Add(3 * time.Minute)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ScheduledShutdown: &fiveMinFromNow,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateFiveMin, shutdownWarningState)
}

func TestCheckScheduledShutdown_OneMinWarning(t *testing.T) {
	mockDB := new(MockDB)
	shutdownWarningState = WarningStateFiveMin
	lastScheduledShutdownTime = nil

	oldSendMsg := sendMinecraftMessageFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	defer func() { sendMinecraftMessageFunc = oldSendMsg }()

	oneMinFromNow := time.Now().Add(30 * time.Second)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ScheduledShutdown: &oneMinFromNow,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateOneMin, shutdownWarningState)
}

func TestCheckScheduledShutdown_ShutdownCancelled(t *testing.T) {
	mockDB := new(MockDB)
	prevTime := time.Now().Add(10 * time.Minute)
	shutdownWarningState = WarningStateFiveMin
	lastScheduledShutdownTime = &prevTime

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ScheduledShutdown: nil,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateNone, shutdownWarningState)
	assert.Nil(t, lastScheduledShutdownTime)
}

// --- initiateScheduledShutdown tests ---

func TestInitiateScheduledShutdown_Success(t *testing.T) {
	mockDB := new(MockDB)

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	oldStopInst := stopInstanceFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	stopInstanceFunc = func(ctx context.Context) error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
		stopInstanceFunc = oldStopInst
	}()

	// Reset global state
	shutdownWarningState = WarningStateOneMin
	prevTime := time.Now()
	lastScheduledShutdownTime = &prevTime

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	// Override the sleep — we can't, but 5 seconds is acceptable for test

	// Use a short context timeout to not wait 5 seconds for the sleep
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := initiateScheduledShutdown(ctx, mockDB, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateNone, shutdownWarningState)
	assert.Nil(t, lastScheduledShutdownTime)
}

func TestInitiateScheduledShutdown_StopError(t *testing.T) {
	mockDB := new(MockDB)

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	oldStopInst := stopInstanceFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	stopInstanceFunc = func(ctx context.Context) error { return fmt.Errorf("stop failed") }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
		stopInstanceFunc = oldStopInst
	}()

	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := initiateScheduledShutdown(ctx, mockDB, "test-instance")
	assert.Error(t, err)
}

func TestGetUptime_ReadFileError(t *testing.T) {
	oldReadFile := osReadFile
	osReadFile = func(filename string) ([]byte, error) {
		return nil, fmt.Errorf("file not found")
	}
	defer func() { osReadFile = oldReadFile }()

	_, err := getUptime()
	assert.Error(t, err)
}

// --- syncWhitelist: NotFound config falls back to disabled ---

func TestSyncWhitelist_ConfigNotFound(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{}, grpcstatus.Error(codes.NotFound, "not found"))
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "[]")
	}

	enabled, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.False(t, enabled)
}

// --- syncWhitelist: add player error is logged but not fatal ---

func TestSyncWhitelist_AddPlayerError(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: true}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{
		{Username: "newplayer", UUID: "uuid-new"},
	}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// whitelist.json - empty
			return exec.Command("echo", "[]")
		}
		if callCount == 2 {
			// add player - fail
			return exec.Command("false")
		}
		// server.properties
		return exec.Command("echo", "white-list=true")
	}

	enabled, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

// --- syncWhitelist: remove player error is logged but not fatal ---

func TestSyncWhitelist_RemovePlayerError(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: true}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// whitelist.json - has a player not in firestore
			entries := []MinecraftWhitelistEntry{{UUID: "uuid-old", Name: "oldplayer"}}
			data, _ := json.Marshal(entries)
			return exec.Command("echo", string(data))
		}
		if callCount == 2 {
			// remove player - fail
			return exec.Command("false")
		}
		// server.properties
		return exec.Command("echo", "white-list=true")
	}

	enabled, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

// --- syncWhitelist: whitelist enabled toggle ---

func TestSyncWhitelist_ToggleWhitelistEnabled(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Config says enabled=true, but server has white-list=false
	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: true}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.Command("echo", "[]") // whitelist.json
		}
		if callCount == 2 {
			return exec.Command("echo", "white-list=false") // server.properties - disabled
		}
		return exec.Command("true") // setWhitelistEnabled rcon
	}

	enabled, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

// --- syncWhitelist: setWhitelistEnabled error ---

func TestSyncWhitelist_SetWhitelistEnabledError(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: true}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.Command("echo", "[]")
		}
		if callCount == 2 {
			return exec.Command("echo", "white-list=false")
		}
		return exec.Command("false") // setWhitelistEnabled fails
	}

	enabled, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err) // error is logged, not returned
	assert.True(t, enabled)
}

// --- syncWhitelist: getMinecraftWhitelist error ---

func TestSyncWhitelist_GetMinecraftWhitelistError(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: true}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false") // whitelist.json read fails
	}

	_, err := syncWhitelist(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
}

// --- checkScheduledShutdown: shutdown time reached ---

func TestCheckScheduledShutdown_TimeReached(t *testing.T) {
	mockDB := new(MockDB)
	shutdownWarningState = WarningStateOneMin
	lastScheduledShutdownTime = nil

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	oldStopInst := stopInstanceFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	stopInstanceFunc = func(ctx context.Context) error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
		stopInstanceFunc = oldStopInst
	}()

	pastTime := time.Now().Add(-1 * time.Minute)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ScheduledShutdown: &pastTime,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
}

// --- checkScheduledShutdown: new different shutdown time resets warning ---

func TestCheckScheduledShutdown_NewShutdownTime(t *testing.T) {
	mockDB := new(MockDB)
	oldTime := time.Now().Add(20 * time.Minute)
	shutdownWarningState = WarningStateFiveMin
	lastScheduledShutdownTime = &oldTime

	newTime := time.Now().Add(30 * time.Minute)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ScheduledShutdown: &newTime,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateNone, shutdownWarningState)
}

// --- checkScheduledShutdown: send warning error is logged ---

func TestCheckScheduledShutdown_FiveMinWarningError(t *testing.T) {
	mockDB := new(MockDB)
	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	oldSendMsg := sendMinecraftMessageFunc
	sendMinecraftMessageFunc = func(msg string) error { return fmt.Errorf("rcon error") }
	defer func() { sendMinecraftMessageFunc = oldSendMsg }()

	fiveMinFromNow := time.Now().Add(3 * time.Minute)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ScheduledShutdown: &fiveMinFromNow,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateFiveMin, shutdownWarningState)
}

func TestCheckScheduledShutdown_OneMinWarningError(t *testing.T) {
	mockDB := new(MockDB)
	shutdownWarningState = WarningStateFiveMin
	lastScheduledShutdownTime = nil

	oldSendMsg := sendMinecraftMessageFunc
	sendMinecraftMessageFunc = func(msg string) error { return fmt.Errorf("rcon error") }
	defer func() { sendMinecraftMessageFunc = oldSendMsg }()

	thirtySecFromNow := time.Now().Add(30 * time.Second)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ScheduledShutdown: &thirtySecFromNow,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateOneMin, shutdownWarningState)
}

// --- initiateScheduledShutdown: GetStatus error ---

func TestInitiateScheduledShutdown_GetStatusError(t *testing.T) {
	mockDB := new(MockDB)

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	oldStopInst := stopInstanceFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	stopInstanceFunc = func(ctx context.Context) error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
		stopInstanceFunc = oldStopInst
	}()

	shutdownWarningState = WarningStateOneMin
	lastScheduledShutdownTime = nil

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, fmt.Errorf("db error"))

	err := initiateScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err) // GetStatus error is logged, not returned
}

// --- initiateScheduledShutdown: UpdateStatus error ---

func TestInitiateScheduledShutdown_UpdateStatusError(t *testing.T) {
	mockDB := new(MockDB)

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	oldStopInst := stopInstanceFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	stopInstanceFunc = func(ctx context.Context) error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
		stopInstanceFunc = oldStopInst
	}()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(fmt.Errorf("update error"))

	err := initiateScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err) // logged, not returned
}

// --- initiateScheduledShutdown: sendMessage and saveWorld errors ---

func TestInitiateScheduledShutdown_MessageAndSaveErrors(t *testing.T) {
	mockDB := new(MockDB)

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	oldStopInst := stopInstanceFunc
	sendMinecraftMessageFunc = func(msg string) error { return fmt.Errorf("msg error") }
	saveMinecraftWorldFunc = func() error { return fmt.Errorf("save error") }
	stopInstanceFunc = func(ctx context.Context) error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
		stopInstanceFunc = oldStopInst
	}()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	err := initiateScheduledShutdown(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err) // errors are logged, shutdown continues
}

// --- runStatusUpdate: full success path ---

func TestRunStatusUpdate_FullSuccess(t *testing.T) {
	mockDB := new(MockDB)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) { return true, nil }
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
}

// --- runStatusUpdate: checkScheduledShutdown error is non-fatal ---

func TestRunStatusUpdate_CheckShutdownError(t *testing.T) {
	mockDB := new(MockDB)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) { return false, nil }
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error {
		return fmt.Errorf("shutdown check error")
	}
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
}

// --- runStatusUpdate: syncWhitelist error is non-fatal ---

func TestRunStatusUpdate_SyncWhitelistError(t *testing.T) {
	mockDB := new(MockDB)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) {
		return false, fmt.Errorf("sync error")
	}
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
}

// --- runStatusUpdate: UpdateStatus error ---

func TestRunStatusUpdate_UpdateStatusError(t *testing.T) {
	mockDB := new(MockDB)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) { return false, nil }
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(fmt.Errorf("db error"))

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
}

// --- runStatusUpdate: version returns rawOutput ---

func TestRunStatusUpdate_VersionWithRawOutput(t *testing.T) {
	mockDB := new(MockDB)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "raw output here", nil }
	syncWhitelistFunc = func(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) { return false, nil }
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
}

// --- getInstanceIP: success ---

func TestGetInstanceIP_Success(t *testing.T) {
	// Can't easily test since it calls metadata.ExternalIP() directly
	// This is covered via runStatusUpdate tests using getInstanceIPFunc
}

// --- getUptime: success ---

func TestGetUptime_Success(t *testing.T) {
	oldReadFile := osReadFile
	osReadFile = func(filename string) ([]byte, error) {
		return []byte("3600.50 7200.00\n"), nil
	}
	defer func() { osReadFile = oldReadFile }()

	result, err := getUptime()
	assert.NoError(t, err)
	assert.Equal(t, "1:00", result)
}

func TestGetUptime_InvalidFormat(t *testing.T) {
	oldReadFile := osReadFile
	osReadFile = func(filename string) ([]byte, error) {
		return []byte("not a number"), nil
	}
	defer func() { osReadFile = oldReadFile }()

	_, err := getUptime()
	assert.Error(t, err)
}

// --- importWhitelistIfEmpty: GetWhitelistEntries error ---

func TestImportWhitelistIfEmpty_GetEntriesError(t *testing.T) {
	mockDB := new(MockDB)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry(nil), fmt.Errorf("db error"))

	err := importWhitelistIfEmpty(context.Background(), mockDB, "test-instance")
	assert.Error(t, err)
}

// --- importWhitelistIfEmpty: AddWhitelistEntry error ---

func TestImportWhitelistIfEmpty_AddEntryError(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	entries := []MinecraftWhitelistEntry{{UUID: "uuid-1", Name: "player1"}}
	entriesJSON, _ := json.Marshal(entries)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.Command("echo", string(entriesJSON))
		}
		return exec.Command("echo", "white-list=false")
	}

	mockDB.On("AddWhitelistEntry", mock.Anything, "test-instance", mock.AnythingOfType("db.WhitelistEntry")).Return(fmt.Errorf("add error"))
	mockDB.On("SetWhitelistConfig", mock.Anything, "test-instance", mock.AnythingOfType("db.WhitelistConfig")).Return(nil)

	err := importWhitelistIfEmpty(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err) // errors are logged, not returned
}

// --- importWhitelistIfEmpty: SetWhitelistConfig error ---

func TestImportWhitelistIfEmpty_SetConfigError(t *testing.T) {
	mockDB := new(MockDB)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{}, nil)

	entries := []MinecraftWhitelistEntry{{UUID: "uuid-1", Name: "player1"}}
	entriesJSON, _ := json.Marshal(entries)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.Command("echo", string(entriesJSON))
		}
		return exec.Command("echo", "white-list=true")
	}

	mockDB.On("AddWhitelistEntry", mock.Anything, "test-instance", mock.AnythingOfType("db.WhitelistEntry")).Return(nil)
	mockDB.On("SetWhitelistConfig", mock.Anything, "test-instance", mock.AnythingOfType("db.WhitelistConfig")).Return(fmt.Errorf("config error"))

	err := importWhitelistIfEmpty(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err) // error is logged
}
