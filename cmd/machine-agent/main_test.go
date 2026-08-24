package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/nbyl/metio/internal/agentclient"
	"github.com/nbyl/metio/internal/dbtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAgentClient struct {
	mock.Mock
}

func (m *MockAgentClient) GetStatus(ctx context.Context) (dbtypes.Status, error) {
	args := m.Called(ctx)
	return args.Get(0).(dbtypes.Status), args.Error(1)
}

func (m *MockAgentClient) UpdateStatus(ctx context.Context, status dbtypes.Status) error {
	args := m.Called(ctx, status)
	return args.Error(0)
}

func (m *MockAgentClient) GetWhitelistEntries(ctx context.Context) ([]dbtypes.WhitelistEntry, error) {
	args := m.Called(ctx)
	return args.Get(0).([]dbtypes.WhitelistEntry), args.Error(1)
}

func (m *MockAgentClient) GetWhitelistConfig(ctx context.Context) (dbtypes.WhitelistConfig, error) {
	args := m.Called(ctx)
	return args.Get(0).(dbtypes.WhitelistConfig), args.Error(1)
}

func (m *MockAgentClient) SetWhitelistConfig(ctx context.Context, cfg dbtypes.WhitelistConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}

func (m *MockAgentClient) AddWhitelistEntry(ctx context.Context, entry dbtypes.WhitelistEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockAgentClient) StopInstance(ctx context.Context, project, zone string) error {
	args := m.Called(ctx, project, zone)
	return args.Error(0)
}

func (m *MockAgentClient) SubmitBackupReport(ctx context.Context, serverID string, report agentclient.BackupReport) error {
	args := m.Called(ctx, serverID, report)
	return args.Error(0)
}

func TestRunStatusUpdate(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncWhitelistFunc := syncWhitelistFunc
	oldCheckFunc := checkScheduledShutdownFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) {
		return 5, 20, nil
	}
	getUptimeFunc = func() (string, error) {
		return "2 days, 3:45", nil
	}
	getMinecraftVersionFunc = func() (string, string, error) {
		return "1.21.4", "", nil
	}
	syncWhitelistFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
		return true, nil
	}
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
		return nil
	}
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncWhitelistFunc
		checkScheduledShutdownFunc = oldCheckFunc
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)

	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil).Run(func(args mock.Arguments) {
		status := args.Get(1).(dbtypes.Status)
		assert.Equal(t, 5, status.Players.Current)
		assert.Equal(t, 20, status.Players.Max)
		assert.Equal(t, dbtypes.ServerStateRunning, status.ServerState)
		assert.Equal(t, "1.21.4", status.Version)
		assert.True(t, status.WhitelistEnabled)
	})

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestRunStatusUpdateError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncWhitelistFunc := syncWhitelistFunc
	oldCheckFunc := checkScheduledShutdownFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) {
		return 0, 10, nil
	}
	getUptimeFunc = func() (string, error) {
		return "2 days, 3:45", nil
	}
	getMinecraftVersionFunc = func() (string, string, error) {
		return "1.21.4", "", nil
	}
	syncWhitelistFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
		return false, nil
	}
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
		return nil
	}
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncWhitelistFunc
		checkScheduledShutdownFunc = oldCheckFunc
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(assert.AnError)

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.Error(t, err)
	mockClient.AssertExpectations(t)
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
			return []byte("172800.00 123456.78"), nil
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
		return exec.Command("false")
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
	mockClient := new(MockAgentClient)
	oldGetFunc := getMinecraftPlayerCountFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) {
		return 0, 0, fmt.Errorf("rcon error")
	}
	defer func() { getMinecraftPlayerCountFunc = oldGetFunc }()

	oldCheck := checkScheduledShutdownFunc
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
		return nil
	}
	defer func() { checkScheduledShutdownFunc = oldCheck }()

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.Error(t, err)
}

func TestRunStatusUpdate_UptimeError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) { return 5, 20, nil }
	getUptimeFunc = func() (string, error) { return "", fmt.Errorf("uptime error") }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
	}()

	oldCheck := checkScheduledShutdownFunc
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error { return nil }
	defer func() { checkScheduledShutdownFunc = oldCheck }()

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.Error(t, err)
}

func TestRunStatusUpdate_VersionError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 5, 20, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "", "", fmt.Errorf("version error") }
	syncWhitelistFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
		return false, nil
	}
	getInstanceIPFunc = func() (string, error) { return "1.2.3.4:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestRunStatusUpdate_IPError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 5, 20, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
		return false, nil
	}
	getInstanceIPFunc = func() (string, error) { return "", fmt.Errorf("metadata error") }
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestImportWhitelistIfEmpty_AlreadyHasEntries(t *testing.T) {
	mockClient := new(MockAgentClient)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{
		{Username: "player1", UUID: "uuid-1"},
	}, nil)

	err := importWhitelistIfEmpty(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestImportWhitelistIfEmpty_EmptyWhitelist(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

	entries := []MinecraftWhitelistEntry{
		{UUID: "uuid-1", Name: "player1"},
		{UUID: "uuid-2", Name: "player2"},
	}
	entriesJSON, _ := json.Marshal(entries)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.Command("echo", string(entriesJSON))
		}
		return exec.Command("echo", "white-list=true")
	}

	mockClient.On("AddWhitelistEntry", mock.Anything, mock.AnythingOfType("dbtypes.WhitelistEntry")).Return(nil)
	mockClient.On("SetWhitelistConfig", mock.Anything, mock.AnythingOfType("dbtypes.WhitelistConfig")).Return(nil)

	err := importWhitelistIfEmpty(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestImportWhitelistIfEmpty_EmptyJSON(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "[]")
	}

	err := importWhitelistIfEmpty(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestImportWhitelistIfEmpty_ReadError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := importWhitelistIfEmpty(context.Background(), mockClient, "test-instance")
	assert.Error(t, err)
}

func TestSyncWhitelist_Success(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{Enabled: true}, nil)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{
		{Username: "player1", UUID: "uuid-1"},
	}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			entries := []MinecraftWhitelistEntry{{UUID: "uuid-1", Name: "player1"}}
			data, _ := json.Marshal(entries)
			return exec.Command("echo", string(data))
		}
		return exec.Command("echo", "white-list=true")
	}

	enabled, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

func TestSyncWhitelist_AddAndRemove(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{Enabled: false}, nil)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{
		{Username: "newplayer", UUID: "uuid-new"},
	}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			entries := []MinecraftWhitelistEntry{{UUID: "uuid-old", Name: "oldplayer"}}
			data, _ := json.Marshal(entries)
			return exec.Command("echo", string(data))
		}
		return exec.Command("true")
	}

	enabled, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.False(t, enabled)
}

func TestSyncWhitelist_GetConfigError(t *testing.T) {
	mockClient := new(MockAgentClient)
	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{}, fmt.Errorf("http error"))
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "[]")
	}
	defer func() { execCommand = oldExec }()

	enabled, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.False(t, enabled)
}

func TestSyncWhitelist_GetEntriesError(t *testing.T) {
	mockClient := new(MockAgentClient)
	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{Enabled: true}, nil)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry(nil), fmt.Errorf("entries error"))

	_, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.Error(t, err)
}

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

func TestCheckScheduledShutdown_NoShutdown(t *testing.T) {
	mockClient := new(MockAgentClient)

	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: nil,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestCheckScheduledShutdown_GetStatusError(t *testing.T) {
	mockClient := new(MockAgentClient)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, fmt.Errorf("http error"))

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.Error(t, err)
}

func TestCheckScheduledShutdown_FutureShutdown_NoWarning(t *testing.T) {
	mockClient := new(MockAgentClient)
	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	futureTime := time.Now().Add(30 * time.Minute)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: &futureTime,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateNone, shutdownWarningState)
}

func TestCheckScheduledShutdown_FiveMinWarning(t *testing.T) {
	mockClient := new(MockAgentClient)
	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	oldSendMsg := sendMinecraftMessageFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	defer func() { sendMinecraftMessageFunc = oldSendMsg }()

	fiveMinFromNow := time.Now().Add(3 * time.Minute)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: &fiveMinFromNow,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateFiveMin, shutdownWarningState)
}

func TestCheckScheduledShutdown_OneMinWarning(t *testing.T) {
	mockClient := new(MockAgentClient)
	shutdownWarningState = WarningStateFiveMin
	lastScheduledShutdownTime = nil

	oldSendMsg := sendMinecraftMessageFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	defer func() { sendMinecraftMessageFunc = oldSendMsg }()

	oneMinFromNow := time.Now().Add(30 * time.Second)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: &oneMinFromNow,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateOneMin, shutdownWarningState)
}

func TestCheckScheduledShutdown_ShutdownCancelled(t *testing.T) {
	mockClient := new(MockAgentClient)
	prevTime := time.Now().Add(10 * time.Minute)
	shutdownWarningState = WarningStateFiveMin
	lastScheduledShutdownTime = &prevTime

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: nil,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateNone, shutdownWarningState)
	assert.Nil(t, lastScheduledShutdownTime)
}

func TestCheckScheduledShutdown_TimeReached(t *testing.T) {
	mockClient := new(MockAgentClient)
	shutdownWarningState = WarningStateOneMin
	lastScheduledShutdownTime = nil
	oldGetProjectID := getProjectIDFunc
	oldGetZone := getZoneFunc
	getProjectIDFunc = func() (string, error) { return "test-project", nil }
	getZoneFunc = func() (string, error) { return "test-zone", nil }
	defer func() {
		getProjectIDFunc = oldGetProjectID
		getZoneFunc = oldGetZone
	}()

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
	}()

	pastTime := time.Now().Add(-1 * time.Minute)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: &pastTime,
	}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)
	mockClient.On("StopInstance", mock.Anything, "test-project", "test-zone").Return(nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestCheckScheduledShutdown_NewShutdownTime(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldTime := time.Now().Add(20 * time.Minute)
	shutdownWarningState = WarningStateFiveMin
	lastScheduledShutdownTime = &oldTime

	newTime := time.Now().Add(30 * time.Minute)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: &newTime,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateNone, shutdownWarningState)
}

func TestCheckScheduledShutdown_FiveMinWarningError(t *testing.T) {
	mockClient := new(MockAgentClient)
	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	oldSendMsg := sendMinecraftMessageFunc
	sendMinecraftMessageFunc = func(msg string) error { return fmt.Errorf("rcon error") }
	defer func() { sendMinecraftMessageFunc = oldSendMsg }()

	fiveMinFromNow := time.Now().Add(3 * time.Minute)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: &fiveMinFromNow,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateFiveMin, shutdownWarningState)
}

func TestCheckScheduledShutdown_OneMinWarningError(t *testing.T) {
	mockClient := new(MockAgentClient)
	shutdownWarningState = WarningStateFiveMin
	lastScheduledShutdownTime = nil

	oldSendMsg := sendMinecraftMessageFunc
	sendMinecraftMessageFunc = func(msg string) error { return fmt.Errorf("rcon error") }
	defer func() { sendMinecraftMessageFunc = oldSendMsg }()

	thirtySecFromNow := time.Now().Add(30 * time.Second)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: &thirtySecFromNow,
	}, nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateOneMin, shutdownWarningState)
}

func TestInitiateScheduledShutdown_Success(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetProjectID := getProjectIDFunc
	oldGetZone := getZoneFunc
	getProjectIDFunc = func() (string, error) { return "test-project", nil }
	getZoneFunc = func() (string, error) { return "test-zone", nil }
	defer func() {
		getProjectIDFunc = oldGetProjectID
		getZoneFunc = oldGetZone
	}()

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
	}()

	shutdownWarningState = WarningStateOneMin
	prevTime := time.Now()
	lastScheduledShutdownTime = &prevTime

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)
	mockClient.On("StopInstance", mock.Anything, "test-project", "test-zone").Return(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := initiateScheduledShutdown(ctx, mockClient, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, WarningStateNone, shutdownWarningState)
	assert.Nil(t, lastScheduledShutdownTime)
}

func TestInitiateScheduledShutdown_StopError(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetProjectID := getProjectIDFunc
	oldGetZone := getZoneFunc
	getProjectIDFunc = func() (string, error) { return "test-project", nil }
	getZoneFunc = func() (string, error) { return "test-zone", nil }
	defer func() {
		getProjectIDFunc = oldGetProjectID
		getZoneFunc = oldGetZone
	}()

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
	}()

	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)
	mockClient.On("StopInstance", mock.Anything, "test-project", "test-zone").Return(fmt.Errorf("stop failed"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := initiateScheduledShutdown(ctx, mockClient, "test-instance")
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

func TestSyncWhitelist_NotFoundConfig(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{}, fmt.Errorf("not found"))
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "[]")
	}

	enabled, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.False(t, enabled)
}

func TestSyncWhitelist_AddPlayerError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{Enabled: true}, nil)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{
		{Username: "newplayer", UUID: "uuid-new"},
	}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.Command("echo", "[]")
		}
		if callCount == 2 {
			return exec.Command("false")
		}
		return exec.Command("echo", "white-list=true")
	}

	enabled, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

func TestSyncWhitelist_RemovePlayerError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{Enabled: true}, nil)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			entries := []MinecraftWhitelistEntry{{UUID: "uuid-old", Name: "oldplayer"}}
			data, _ := json.Marshal(entries)
			return exec.Command("echo", string(data))
		}
		if callCount == 2 {
			return exec.Command("false")
		}
		return exec.Command("echo", "white-list=true")
	}

	enabled, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

func TestSyncWhitelist_ToggleWhitelistEnabled(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{Enabled: true}, nil)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.Command("echo", "[]")
		}
		if callCount == 2 {
			return exec.Command("echo", "white-list=false")
		}
		return exec.Command("true")
	}

	enabled, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

func TestSyncWhitelist_SetWhitelistEnabledError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{Enabled: true}, nil)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

	callCount := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.Command("echo", "[]")
		}
		if callCount == 2 {
			return exec.Command("echo", "white-list=false")
		}
		return exec.Command("false")
	}

	enabled, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

func TestSyncWhitelist_GetMinecraftWhitelistError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistConfig", mock.Anything).Return(dbtypes.WhitelistConfig{Enabled: true}, nil)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	_, err := syncWhitelist(context.Background(), mockClient, "test-instance")
	assert.Error(t, err)
}

func TestCheckScheduledShutdown_NewShutdownTimeResetsWarningAndTimeReached(t *testing.T) {
	mockClient := new(MockAgentClient)
	shutdownWarningState = WarningStateOneMin
	lastScheduledShutdownTime = nil
	oldGetProjectID := getProjectIDFunc
	oldGetZone := getZoneFunc
	getProjectIDFunc = func() (string, error) { return "test-project", nil }
	getZoneFunc = func() (string, error) { return "test-zone", nil }
	defer func() {
		getProjectIDFunc = oldGetProjectID
		getZoneFunc = oldGetZone
	}()

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
	}()

	pastTime := time.Now().Add(-1 * time.Minute)
	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{
		ScheduledShutdown: &pastTime,
	}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)
	mockClient.On("StopInstance", mock.Anything, "test-project", "test-zone").Return(nil)

	err := checkScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestInitiateScheduledShutdown_GetStatusError(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetProjectID := getProjectIDFunc
	oldGetZone := getZoneFunc
	getProjectIDFunc = func() (string, error) { return "test-project", nil }
	getZoneFunc = func() (string, error) { return "test-zone", nil }
	defer func() {
		getProjectIDFunc = oldGetProjectID
		getZoneFunc = oldGetZone
	}()

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
	}()

	shutdownWarningState = WarningStateOneMin
	lastScheduledShutdownTime = nil

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, fmt.Errorf("http error"))
	mockClient.On("StopInstance", mock.Anything, "test-project", "test-zone").Return(nil)

	err := initiateScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestInitiateScheduledShutdown_UpdateStatusError(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetProjectID := getProjectIDFunc
	oldGetZone := getZoneFunc
	getProjectIDFunc = func() (string, error) { return "test-project", nil }
	getZoneFunc = func() (string, error) { return "test-zone", nil }
	defer func() {
		getProjectIDFunc = oldGetProjectID
		getZoneFunc = oldGetZone
	}()

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	sendMinecraftMessageFunc = func(msg string) error { return nil }
	saveMinecraftWorldFunc = func() error { return nil }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(fmt.Errorf("update error"))
	mockClient.On("StopInstance", mock.Anything, "test-project", "test-zone").Return(nil)

	err := initiateScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestInitiateScheduledShutdown_MessageAndSaveErrors(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetProjectID := getProjectIDFunc
	oldGetZone := getZoneFunc
	getProjectIDFunc = func() (string, error) { return "test-project", nil }
	getZoneFunc = func() (string, error) { return "test-zone", nil }
	defer func() {
		getProjectIDFunc = oldGetProjectID
		getZoneFunc = oldGetZone
	}()

	oldSendMsg := sendMinecraftMessageFunc
	oldSaveWorld := saveMinecraftWorldFunc
	sendMinecraftMessageFunc = func(msg string) error { return fmt.Errorf("msg error") }
	saveMinecraftWorldFunc = func() error { return fmt.Errorf("save error") }
	defer func() {
		sendMinecraftMessageFunc = oldSendMsg
		saveMinecraftWorldFunc = oldSaveWorld
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)
	mockClient.On("StopInstance", mock.Anything, "test-project", "test-zone").Return(nil)

	err := initiateScheduledShutdown(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestRunStatusUpdate_FullSuccess(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
		return true, nil
	}
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestRunStatusUpdate_CheckShutdownError(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
		return false, nil
	}
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
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

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestRunStatusUpdate_SyncWhitelistError(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
		return false, fmt.Errorf("sync error")
	}
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestRunStatusUpdate_UpdateStatusError(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "", nil }
	syncWhitelistFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
		return false, nil
	}
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(fmt.Errorf("http error"))

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.Error(t, err)
}

func TestRunStatusUpdate_VersionWithRawOutput(t *testing.T) {
	mockClient := new(MockAgentClient)

	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	oldVersionFunc := getMinecraftVersionFunc
	oldSyncFunc := syncWhitelistFunc
	oldIPFunc := getInstanceIPFunc
	oldCheck := checkScheduledShutdownFunc

	getMinecraftPlayerCountFunc = func() (int, int, error) { return 3, 10, nil }
	getUptimeFunc = func() (string, error) { return "1:00", nil }
	getMinecraftVersionFunc = func() (string, string, error) { return "1.21.4", "raw output here", nil }
	syncWhitelistFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
		return false, nil
	}
	getInstanceIPFunc = func() (string, error) { return "10.0.0.1:25565", nil }
	checkScheduledShutdownFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error { return nil }
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
		getMinecraftVersionFunc = oldVersionFunc
		syncWhitelistFunc = oldSyncFunc
		getInstanceIPFunc = oldIPFunc
		checkScheduledShutdownFunc = oldCheck
	}()

	mockClient.On("GetStatus", mock.Anything).Return(dbtypes.Status{}, nil)
	mockClient.On("UpdateStatus", mock.Anything, mock.AnythingOfType("dbtypes.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestGetInstanceIP_Success(t *testing.T) {
}

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

func TestImportWhitelistIfEmpty_GetEntriesError(t *testing.T) {
	mockClient := new(MockAgentClient)
	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry(nil), fmt.Errorf("http error"))

	err := importWhitelistIfEmpty(context.Background(), mockClient, "test-instance")
	assert.Error(t, err)
}

func TestImportWhitelistIfEmpty_AddEntryError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

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

	mockClient.On("AddWhitelistEntry", mock.Anything, mock.AnythingOfType("dbtypes.WhitelistEntry")).Return(fmt.Errorf("add error"))
	mockClient.On("SetWhitelistConfig", mock.Anything, mock.AnythingOfType("dbtypes.WhitelistConfig")).Return(nil)

	err := importWhitelistIfEmpty(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}

func TestImportWhitelistIfEmpty_SetConfigError(t *testing.T) {
	mockClient := new(MockAgentClient)
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	mockClient.On("GetWhitelistEntries", mock.Anything).Return([]dbtypes.WhitelistEntry{}, nil)

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

	mockClient.On("AddWhitelistEntry", mock.Anything, mock.AnythingOfType("dbtypes.WhitelistEntry")).Return(nil)
	mockClient.On("SetWhitelistConfig", mock.Anything, mock.AnythingOfType("dbtypes.WhitelistConfig")).Return(fmt.Errorf("config error"))

	err := importWhitelistIfEmpty(context.Background(), mockClient, "test-instance")
	assert.NoError(t, err)
}
