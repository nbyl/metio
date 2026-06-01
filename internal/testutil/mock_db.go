package testutil

import (
	"context"

	"github.com/stretchr/testify/mock"
	"gitlab.com/nbyl/metio/internal/db"
)

// MockDB is a shared testify mock implementation of the db.DB interface.
// Use this instead of creating per-package MockDB copies.
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

func (m *MockDB) CreateServerConfig(ctx context.Context, serverID string, config *db.ServerConfig) error {
	args := m.Called(ctx, serverID, config)
	return args.Error(0)
}

func (m *MockDB) GetServerConfig(ctx context.Context, serverID string) (*db.ServerConfig, error) {
	args := m.Called(ctx, serverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.ServerConfig), args.Error(1)
}

func (m *MockDB) UpdateServerConfig(ctx context.Context, serverID string, config *db.ServerConfig) error {
	args := m.Called(ctx, serverID, config)
	return args.Error(0)
}

func (m *MockDB) DeleteServerConfig(ctx context.Context, serverID string) error {
	args := m.Called(ctx, serverID)
	return args.Error(0)
}

func (m *MockDB) ListServerConfigs(ctx context.Context) ([]*db.ServerConfig, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.ServerConfig), args.Error(1)
}

func (m *MockDB) GetPulumiSettings(ctx context.Context) (*db.PulumiSettings, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.PulumiSettings), args.Error(1)
}

func (m *MockDB) SetPulumiSettings(ctx context.Context, settings *db.PulumiSettings) error {
	args := m.Called(ctx, settings)
	return args.Error(0)
}

func (m *MockDB) SaveConfigSnapshot(ctx context.Context, serverID string, config *db.ServerConfig) error {
	args := m.Called(ctx, serverID, config)
	return args.Error(0)
}

func (m *MockDB) GetConfigSnapshot(ctx context.Context, serverID string) (*db.ServerConfig, error) {
	args := m.Called(ctx, serverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.ServerConfig), args.Error(1)
}

func (m *MockDB) DeleteConfigSnapshot(ctx context.Context, serverID string) error {
	args := m.Called(ctx, serverID)
	return args.Error(0)
}
