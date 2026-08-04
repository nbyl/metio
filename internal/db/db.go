package db

import "context"

type DB interface {
	UpdateStatus(ctx context.Context, instanceName string, status Status) error
	GetStatus(ctx context.Context, instanceName string) (Status, error)
	GetWhitelistConfig(ctx context.Context, instanceName string) (WhitelistConfig, error)
	SetWhitelistConfig(ctx context.Context, instanceName string, config WhitelistConfig) error
	GetWhitelistEntries(ctx context.Context, instanceName string) ([]WhitelistEntry, error)
	AddWhitelistEntry(ctx context.Context, instanceName string, entry WhitelistEntry) error
	RemoveWhitelistEntry(ctx context.Context, instanceName string, uuid string) error
	SetWhitelistEntries(ctx context.Context, instanceName string, entries []WhitelistEntry) error
	GetProvisioningStatus(ctx context.Context, serverID string) (*ProvisioningStatus, error)
	UpdateProvisioningStatus(ctx context.Context, serverID string, status *ProvisioningStatus) error
	AddProvisioningStep(ctx context.Context, serverID string, step ProvisioningStep) error
	CompleteProvisioning(ctx context.Context, serverID string, outputs map[string]string) error
	FailProvisioning(ctx context.Context, serverID string, errMsg string) error
	CreateServerConfig(ctx context.Context, serverID string, config *ServerConfig) error
	GetServerConfig(ctx context.Context, serverID string) (*ServerConfig, error)
	UpdateServerConfig(ctx context.Context, serverID string, config *ServerConfig) error
	DeleteServerConfig(ctx context.Context, serverID string) error
	ListServerConfigs(ctx context.Context) ([]*ServerConfig, error)
	SaveConfigSnapshot(ctx context.Context, serverID string, config *ServerConfig) error
	GetConfigSnapshot(ctx context.Context, serverID string) (*ServerConfig, error)
	DeleteConfigSnapshot(ctx context.Context, serverID string) error
	GetPulumiSettings(ctx context.Context) (*PulumiSettings, error)
	SetPulumiSettings(ctx context.Context, settings *PulumiSettings) error
	ListAllServerIDs(ctx context.Context) ([]string, error)
}
