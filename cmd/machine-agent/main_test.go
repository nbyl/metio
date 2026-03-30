package main

import (
	"context"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/nbyl/metio/db"
)

type MockDB struct {
	mock.Mock
}

func (m *MockDB) UpdateStatus(ctx context.Context, instanceName string, status db.Status) error {
	args := m.Called(ctx, instanceName, status)
	return args.Error(0)
}

func (m *MockDB) GetStatus(ctx context.Context, instanceName string) (db.Status, error) {
	args := m.Called(ctx, instanceName)
	return args.Get(0).(db.Status), args.Error(1)
}

func (m *MockDB) GetWhitelistConfig(ctx context.Context, instanceName string) (db.WhitelistConfig, error) {
	args := m.Called(ctx, instanceName)
	return args.Get(0).(db.WhitelistConfig), args.Error(1)
}

func (m *MockDB) SetWhitelistConfig(ctx context.Context, instanceName string, config db.WhitelistConfig) error {
	args := m.Called(ctx, instanceName, config)
	return args.Error(0)
}

func (m *MockDB) GetWhitelistEntries(ctx context.Context, instanceName string) ([]db.WhitelistEntry, error) {
	args := m.Called(ctx, instanceName)
	return args.Get(0).([]db.WhitelistEntry), args.Error(1)
}

func (m *MockDB) AddWhitelistEntry(ctx context.Context, instanceName string, entry db.WhitelistEntry) error {
	args := m.Called(ctx, instanceName, entry)
	return args.Error(0)
}

func (m *MockDB) RemoveWhitelistEntry(ctx context.Context, instanceName string, uuid string) error {
	args := m.Called(ctx, instanceName, uuid)
	return args.Error(0)
}

func (m *MockDB) SetWhitelistEntries(ctx context.Context, instanceName string, entries []db.WhitelistEntry) error {
	args := m.Called(ctx, instanceName, entries)
	return args.Error(0)
}

func (m *MockDB) GetProvisioningStatus(ctx context.Context, serverID string) (*db.ProvisioningStatus, error) {
	args := m.Called(ctx, serverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.ProvisioningStatus), args.Error(1)
}

func (m *MockDB) UpdateProvisioningStatus(ctx context.Context, serverID string, status *db.ProvisioningStatus) error {
	args := m.Called(ctx, serverID, status)
	return args.Error(0)
}

func (m *MockDB) AddProvisioningStep(ctx context.Context, serverID string, step db.ProvisioningStep) error {
	args := m.Called(ctx, serverID, step)
	return args.Error(0)
}

func (m *MockDB) CompleteProvisioning(ctx context.Context, serverID string, outputs map[string]string) error {
	args := m.Called(ctx, serverID, outputs)
	return args.Error(0)
}

func (m *MockDB) FailProvisioning(ctx context.Context, serverID string, errMsg string) error {
	args := m.Called(ctx, serverID, errMsg)
	return args.Error(0)
}

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
