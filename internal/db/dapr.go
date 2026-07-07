package db

import (
	"context"
	"fmt"
	"log"
)

type DaprDB struct {
	stateStoreName string
}

func NewDaprDB(ctx context.Context, stateStoreName string) (DB, error) {
	log.Printf("DaprDB: initializing with state store %q (stub)", stateStoreName)
	return &DaprDB{stateStoreName: stateStoreName}, nil
}

func (d *DaprDB) UpdateStatus(ctx context.Context, instanceName string, status Status) error {
	return fmt.Errorf("DaprDB.UpdateStatus not yet implemented")
}

func (d *DaprDB) GetStatus(ctx context.Context, instanceName string) (Status, error) {
	return Status{}, fmt.Errorf("DaprDB.GetStatus not yet implemented")
}

func (d *DaprDB) GetWhitelistConfig(ctx context.Context, instanceName string) (WhitelistConfig, error) {
	return WhitelistConfig{}, fmt.Errorf("DaprDB.GetWhitelistConfig not yet implemented")
}

func (d *DaprDB) SetWhitelistConfig(ctx context.Context, instanceName string, config WhitelistConfig) error {
	return fmt.Errorf("DaprDB.SetWhitelistConfig not yet implemented")
}

func (d *DaprDB) GetWhitelistEntries(ctx context.Context, instanceName string) ([]WhitelistEntry, error) {
	return nil, fmt.Errorf("DaprDB.GetWhitelistEntries not yet implemented")
}

func (d *DaprDB) AddWhitelistEntry(ctx context.Context, instanceName string, entry WhitelistEntry) error {
	return fmt.Errorf("DaprDB.AddWhitelistEntry not yet implemented")
}

func (d *DaprDB) RemoveWhitelistEntry(ctx context.Context, instanceName string, uuid string) error {
	return fmt.Errorf("DaprDB.RemoveWhitelistEntry not yet implemented")
}

func (d *DaprDB) SetWhitelistEntries(ctx context.Context, instanceName string, entries []WhitelistEntry) error {
	return fmt.Errorf("DaprDB.SetWhitelistEntries not yet implemented")
}

func (d *DaprDB) GetProvisioningStatus(ctx context.Context, serverID string) (*ProvisioningStatus, error) {
	return nil, fmt.Errorf("DaprDB.GetProvisioningStatus not yet implemented")
}

func (d *DaprDB) UpdateProvisioningStatus(ctx context.Context, serverID string, status *ProvisioningStatus) error {
	return fmt.Errorf("DaprDB.UpdateProvisioningStatus not yet implemented")
}

func (d *DaprDB) AddProvisioningStep(ctx context.Context, serverID string, step ProvisioningStep) error {
	return fmt.Errorf("DaprDB.AddProvisioningStep not yet implemented")
}

func (d *DaprDB) CompleteProvisioning(ctx context.Context, serverID string, outputs map[string]string) error {
	return fmt.Errorf("DaprDB.CompleteProvisioning not yet implemented")
}

func (d *DaprDB) FailProvisioning(ctx context.Context, serverID string, errMsg string) error {
	return fmt.Errorf("DaprDB.FailProvisioning not yet implemented")
}

func (d *DaprDB) CreateServerConfig(ctx context.Context, serverID string, config *ServerConfig) error {
	return fmt.Errorf("DaprDB.CreateServerConfig not yet implemented")
}

func (d *DaprDB) GetServerConfig(ctx context.Context, serverID string) (*ServerConfig, error) {
	return nil, fmt.Errorf("DaprDB.GetServerConfig not yet implemented")
}

func (d *DaprDB) UpdateServerConfig(ctx context.Context, serverID string, config *ServerConfig) error {
	return fmt.Errorf("DaprDB.UpdateServerConfig not yet implemented")
}

func (d *DaprDB) DeleteServerConfig(ctx context.Context, serverID string) error {
	return fmt.Errorf("DaprDB.DeleteServerConfig not yet implemented")
}

func (d *DaprDB) ListServerConfigs(ctx context.Context) ([]*ServerConfig, error) {
	return nil, fmt.Errorf("DaprDB.ListServerConfigs not yet implemented")
}

func (d *DaprDB) SaveConfigSnapshot(ctx context.Context, serverID string, config *ServerConfig) error {
	return fmt.Errorf("DaprDB.SaveConfigSnapshot not yet implemented")
}

func (d *DaprDB) GetConfigSnapshot(ctx context.Context, serverID string) (*ServerConfig, error) {
	return nil, fmt.Errorf("DaprDB.GetConfigSnapshot not yet implemented")
}

func (d *DaprDB) DeleteConfigSnapshot(ctx context.Context, serverID string) error {
	return fmt.Errorf("DaprDB.DeleteConfigSnapshot not yet implemented")
}

func (d *DaprDB) GetPulumiSettings(ctx context.Context) (*PulumiSettings, error) {
	return nil, fmt.Errorf("DaprDB.GetPulumiSettings not yet implemented")
}

func (d *DaprDB) SetPulumiSettings(ctx context.Context, settings *PulumiSettings) error {
	return fmt.Errorf("DaprDB.SetPulumiSettings not yet implemented")
}

func (d *DaprDB) ListAllServerIDs(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("DaprDB.ListAllServerIDs not yet implemented")
}
