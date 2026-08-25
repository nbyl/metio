package testutil

import (
	"context"

	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/pulumi/programs"
	"github.com/stretchr/testify/mock"
)

// MockProvisioningService implements handlers.ProvisioningServiceInterface for testing.
type MockProvisioningService struct {
	mock.Mock
}

func (m *MockProvisioningService) CreateServer(ctx context.Context, serverID string, config *programs.ServerConfig) error {
	args := m.Called(ctx, serverID, config)
	return args.Error(0)
}

func (m *MockProvisioningService) CreateServerFromBackup(ctx context.Context, serverID string, config *programs.ServerConfig) error {
	args := m.Called(ctx, serverID, config)
	return args.Error(0)
}

func (m *MockProvisioningService) UpdateServer(ctx context.Context, serverID string, config *programs.ServerConfig, updateType int) error {
	args := m.Called(ctx, serverID, config, updateType)
	return args.Error(0)
}

func (m *MockProvisioningService) DestroyServer(ctx context.Context, serverID string) error {
	args := m.Called(ctx, serverID)
	return args.Error(0)
}

func (m *MockProvisioningService) RestoreServer(ctx context.Context, serverID string, backup *db.Backup, versionWarning string) error {
	args := m.Called(ctx, serverID, backup, versionWarning)
	return args.Error(0)
}

func (m *MockProvisioningService) GetProvisioningStatus(ctx context.Context, serverID string) (*db.ProvisioningStatus, error) {
	args := m.Called(ctx, serverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.ProvisioningStatus), args.Error(1)
}

func (m *MockProvisioningService) RevertServerConfig(ctx context.Context, serverID string) error {
	args := m.Called(ctx, serverID)
	return args.Error(0)
}
