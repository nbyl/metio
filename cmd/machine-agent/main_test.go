package main

import (
	"context"
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

func TestRunStatusUpdate(t *testing.T) {
	mockDB := new(MockDB)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) {
		return 5, 20, nil
	}
	getUptimeFunc = func() (string, error) {
		return "2 days, 3:45", nil
	}
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
	}()

	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	err := runStatusUpdate(context.Background(), mockDB, "test-instance")
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestRunStatusUpdateError(t *testing.T) {
	mockDB := new(MockDB)
	oldGetFunc := getMinecraftPlayerCountFunc
	oldUptimeFunc := getUptimeFunc
	getMinecraftPlayerCountFunc = func() (int, int, error) {
		return 0, 10, nil
	}
	getUptimeFunc = func() (string, error) {
		return "2 days, 3:45", nil
	}
	defer func() {
		getMinecraftPlayerCountFunc = oldGetFunc
		getUptimeFunc = oldUptimeFunc
	}()

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
	oldExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "14:23:45 up 2 days, 3:45, 1 user, load average: 0.00, 0.00, 0.00")
	}
	defer func() { execCommand = oldExecCommand }()

	uptime, err := getUptime()
	assert.NoError(t, err)
	assert.Equal(t, "2 days, 3:45", uptime)
}

func TestGetUptimeInvalidOutput(t *testing.T) {
	oldExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "Invalid output")
	}
	defer func() { execCommand = oldExecCommand }()

	_, err := getUptime()
	assert.Error(t, err)
}
