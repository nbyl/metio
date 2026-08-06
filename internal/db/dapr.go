package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	dapr "github.com/dapr/go-sdk/client"
)

type daprStateItem struct {
	Key   string
	Value []byte
}

type daprBulkStateItem struct {
	Key   string
	Value []byte
	Error string
}

type daprClient interface {
	Save(ctx context.Context, storeName, key string, data []byte) error
	Get(ctx context.Context, storeName, key string) (*daprStateItem, error)
	GetBulk(ctx context.Context, storeName string, keys []string) ([]*daprBulkStateItem, error)
	Delete(ctx context.Context, storeName, key string) error
}

type daprClientAdapter struct {
	client dapr.Client
}

func (a *daprClientAdapter) Save(ctx context.Context, storeName, key string, data []byte) error {
	return a.client.SaveState(ctx, storeName, key, data, nil)
}

func (a *daprClientAdapter) Get(ctx context.Context, storeName, key string) (*daprStateItem, error) {
	item, err := a.client.GetState(ctx, storeName, key, nil)
	if err != nil {
		return nil, err
	}
	if item == nil || item.Value == nil {
		return nil, nil
	}
	return &daprStateItem{Key: item.Key, Value: item.Value}, nil
}

func (a *daprClientAdapter) GetBulk(ctx context.Context, storeName string, keys []string) ([]*daprBulkStateItem, error) {
	items, err := a.client.GetBulkState(ctx, storeName, keys, nil, 10)
	if err != nil {
		return nil, err
	}
	result := make([]*daprBulkStateItem, len(items))
	for i, item := range items {
		result[i] = &daprBulkStateItem{
			Key:   item.Key,
			Value: item.Value,
			Error: item.Error,
		}
	}
	return result, nil
}

func (a *daprClientAdapter) Delete(ctx context.Context, storeName, key string) error {
	return a.client.DeleteState(ctx, storeName, key, nil)
}

type whitelistIndex struct {
	UUIDs []string `json:"uuids"`
}

type serverIndex struct {
	ServerIDs []string `json:"server_ids"`
}

type DaprDB struct {
	client         daprClient
	stateStoreName string
}

func NewDaprDB(ctx context.Context, stateStoreName string) (DB, error) {
	daprClient, err := dapr.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Dapr client: %w", err)
	}
	adapter := &daprClientAdapter{client: daprClient}
	log.Printf("DaprDB: initialized with state store %q", stateStoreName)
	return &DaprDB{client: adapter, stateStoreName: stateStoreName}, nil
}

func keySuffix(key string) string {
	_, suffix, _ := strings.Cut(key, ":")
	return suffix
}

func (d *DaprDB) getWhitelistIndex(ctx context.Context, instanceName string) ([]string, error) {
	key := fmt.Sprintf("whitelistidx:%s", instanceName)
	item, err := d.client.Get(ctx, d.stateStoreName, key)
	if err != nil {
		return nil, err
	}
	if item == nil || item.Value == nil {
		return []string{}, nil
	}
	var idx whitelistIndex
	if err := json.Unmarshal(item.Value, &idx); err != nil {
		return nil, err
	}
	return idx.UUIDs, nil
}

func (d *DaprDB) saveWhitelistIndex(ctx context.Context, instanceName string, uuids []string) error {
	key := fmt.Sprintf("whitelistidx:%s", instanceName)
	data, err := json.Marshal(whitelistIndex{UUIDs: uuids})
	if err != nil {
		return err
	}
	return d.client.Save(ctx, d.stateStoreName, key, data)
}

func (d *DaprDB) getServerIndex(ctx context.Context) ([]string, error) {
	item, err := d.client.Get(ctx, d.stateStoreName, "serverindex")
	if err != nil {
		return nil, err
	}
	if item == nil || item.Value == nil {
		return []string{}, nil
	}
	var idx serverIndex
	if err := json.Unmarshal(item.Value, &idx); err != nil {
		return nil, err
	}
	return idx.ServerIDs, nil
}

func (d *DaprDB) saveServerIndex(ctx context.Context, serverIDs []string) error {
	data, err := json.Marshal(serverIndex{ServerIDs: serverIDs})
	if err != nil {
		return err
	}
	return d.client.Save(ctx, d.stateStoreName, "serverindex", data)
}

func (d *DaprDB) UpdateStatus(ctx context.Context, instanceName string, status Status) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("status:%s", instanceName)
	if err := d.client.Save(ctx, d.stateStoreName, key, data); err != nil {
		return err
	}
	log.Printf("DaprDB: updated status for instance %s", instanceName)
	return nil
}

func (d *DaprDB) GetStatus(ctx context.Context, instanceName string) (Status, error) {
	key := fmt.Sprintf("status:%s", instanceName)
	item, err := d.client.Get(ctx, d.stateStoreName, key)
	if err != nil {
		return Status{}, err
	}
	if item == nil || item.Value == nil {
		return Status{}, fmt.Errorf("%w: status not found for %q", ErrNotFound, instanceName)
	}
	var status Status
	if err := json.Unmarshal(item.Value, &status); err != nil {
		return Status{}, err
	}
	return status, nil
}

func (d *DaprDB) GetWhitelistConfig(ctx context.Context, instanceName string) (WhitelistConfig, error) {
	key := fmt.Sprintf("whitelistcfg:%s", instanceName)
	item, err := d.client.Get(ctx, d.stateStoreName, key)
	if err != nil {
		return WhitelistConfig{}, err
	}
	if item == nil || item.Value == nil {
		return WhitelistConfig{}, fmt.Errorf("%w: whitelist config not found for %q", ErrNotFound, instanceName)
	}
	var config WhitelistConfig
	if err := json.Unmarshal(item.Value, &config); err != nil {
		return WhitelistConfig{}, err
	}
	return config, nil
}

func (d *DaprDB) SetWhitelistConfig(ctx context.Context, instanceName string, config WhitelistConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("whitelistcfg:%s", instanceName)
	if err := d.client.Save(ctx, d.stateStoreName, key, data); err != nil {
		return err
	}
	log.Printf("DaprDB: updated whitelist config for instance %s", instanceName)
	return nil
}

func (d *DaprDB) GetWhitelistEntries(ctx context.Context, instanceName string) ([]WhitelistEntry, error) {
	uuids, err := d.getWhitelistIndex(ctx, instanceName)
	if err != nil {
		return nil, err
	}
	if len(uuids) == 0 {
		return []WhitelistEntry{}, nil
	}

	keys := make([]string, len(uuids))
	for i, uuid := range uuids {
		keys[i] = fmt.Sprintf("whitelist:%s:%s", instanceName, uuid)
	}

	items, err := d.client.GetBulk(ctx, d.stateStoreName, keys)
	if err != nil {
		return nil, err
	}

	entries := make([]WhitelistEntry, 0, len(items))
	for _, item := range items {
		if item.Error != "" {
			log.Printf("DaprDB.GetWhitelistEntries: error for key %q: %s", item.Key, item.Error)
			continue
		}
		if item.Value == nil {
			continue
		}
		var entry WhitelistEntry
		if err := json.Unmarshal(item.Value, &entry); err != nil {
			log.Printf("DaprDB.GetWhitelistEntries: error unmarshaling %q: %v", item.Key, err)
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (d *DaprDB) AddWhitelistEntry(ctx context.Context, instanceName string, entry WhitelistEntry) error {
	key := fmt.Sprintf("whitelist:%s:%s", instanceName, entry.UUID)
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := d.client.Save(ctx, d.stateStoreName, key, data); err != nil {
		return err
	}

	idx, err := d.getWhitelistIndex(ctx, instanceName)
	if err != nil {
		return err
	}

	for _, uuid := range idx {
		if uuid == entry.UUID {
			log.Printf("DaprDB: %s already in whitelist index for %s", entry.UUID, instanceName)
			return nil
		}
	}

	idx = append(idx, entry.UUID)
	log.Printf("DaprDB: added %s to whitelist for instance %s", entry.Username, instanceName)
	return d.saveWhitelistIndex(ctx, instanceName, idx)
}

func (d *DaprDB) RemoveWhitelistEntry(ctx context.Context, instanceName string, uuid string) error {
	key := fmt.Sprintf("whitelist:%s:%s", instanceName, uuid)
	if err := d.client.Delete(ctx, d.stateStoreName, key); err != nil {
		return err
	}

	idx, err := d.getWhitelistIndex(ctx, instanceName)
	if err != nil {
		return err
	}

	newIdx := make([]string, 0, len(idx))
	for _, id := range idx {
		if id != uuid {
			newIdx = append(newIdx, id)
		}
	}

	log.Printf("DaprDB: removed player with UUID %s from whitelist for instance %s", uuid, instanceName)
	return d.saveWhitelistIndex(ctx, instanceName, newIdx)
}

func (d *DaprDB) SetWhitelistEntries(ctx context.Context, instanceName string, entries []WhitelistEntry) error {
	oldUUIDs, err := d.getWhitelistIndex(ctx, instanceName)
	if err != nil {
		return err
	}

	for _, uuid := range oldUUIDs {
		key := fmt.Sprintf("whitelist:%s:%s", instanceName, uuid)
		if err := d.client.Delete(ctx, d.stateStoreName, key); err != nil {
			log.Printf("DaprDB.SetWhitelistEntries: error deleting %q: %v", key, err)
		}
	}

	newUUIDs := make([]string, len(entries))
	for i, entry := range entries {
		key := fmt.Sprintf("whitelist:%s:%s", instanceName, entry.UUID)
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if err := d.client.Save(ctx, d.stateStoreName, key, data); err != nil {
			return err
		}
		newUUIDs[i] = entry.UUID
	}

	log.Printf("DaprDB: set %d whitelist entries for instance %s", len(entries), instanceName)
	return d.saveWhitelistIndex(ctx, instanceName, newUUIDs)
}

func (d *DaprDB) GetProvisioningStatus(ctx context.Context, serverID string) (*ProvisioningStatus, error) {
	key := fmt.Sprintf("provisioning:%s", serverID)
	item, err := d.client.Get(ctx, d.stateStoreName, key)
	if err != nil {
		return nil, err
	}
	if item == nil || item.Value == nil {
		return nil, fmt.Errorf("%w: provisioning status not found for %q", ErrNotFound, serverID)
	}
	var status ProvisioningStatus
	if err := json.Unmarshal(item.Value, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (d *DaprDB) UpdateProvisioningStatus(ctx context.Context, serverID string, status *ProvisioningStatus) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("provisioning:%s", serverID)
	return d.client.Save(ctx, d.stateStoreName, key, data)
}

func (d *DaprDB) AddProvisioningStep(ctx context.Context, serverID string, step ProvisioningStep) error {
	status, err := d.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		return err
	}
	status.Steps = append(status.Steps, step)
	status.CurrentStep = step.Name
	return d.UpdateProvisioningStatus(ctx, serverID, status)
}

func (d *DaprDB) CompleteProvisioning(ctx context.Context, serverID string, outputs map[string]string) error {
	status, err := d.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		return err
	}
	now := time.Now()
	status.State = ProvisioningStateCompleted
	status.CompletedAt = &now
	status.Outputs = outputs
	return d.UpdateProvisioningStatus(ctx, serverID, status)
}

func (d *DaprDB) FailProvisioning(ctx context.Context, serverID string, errMsg string) error {
	status, err := d.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		return err
	}
	now := time.Now()
	status.State = ProvisioningStateFailed
	status.CompletedAt = &now
	status.Error = errMsg
	return d.UpdateProvisioningStatus(ctx, serverID, status)
}

func (d *DaprDB) CreateServerConfig(ctx context.Context, serverID string, config *ServerConfig) error {
	key := fmt.Sprintf("serverconfig:%s", serverID)
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := d.client.Save(ctx, d.stateStoreName, key, data); err != nil {
		return err
	}

	idx, err := d.getServerIndex(ctx)
	if err != nil {
		return err
	}

	for _, id := range idx {
		if id == serverID {
			return nil
		}
	}

	idx = append(idx, serverID)
	return d.saveServerIndex(ctx, idx)
}

func (d *DaprDB) GetServerConfig(ctx context.Context, serverID string) (*ServerConfig, error) {
	key := fmt.Sprintf("serverconfig:%s", serverID)
	item, err := d.client.Get(ctx, d.stateStoreName, key)
	if err != nil {
		return nil, err
	}
	if item == nil || item.Value == nil {
		return nil, fmt.Errorf("%w: server config not found for %q", ErrNotFound, serverID)
	}
	var config ServerConfig
	if err := json.Unmarshal(item.Value, &config); err != nil {
		return nil, err
	}
	config.ID = serverID
	return &config, nil
}

func (d *DaprDB) UpdateServerConfig(ctx context.Context, serverID string, config *ServerConfig) error {
	key := fmt.Sprintf("serverconfig:%s", serverID)
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return d.client.Save(ctx, d.stateStoreName, key, data)
}

func (d *DaprDB) DeleteServerConfig(ctx context.Context, serverID string) error {
	key := fmt.Sprintf("serverconfig:%s", serverID)
	if err := d.client.Delete(ctx, d.stateStoreName, key); err != nil {
		return err
	}

	idx, err := d.getServerIndex(ctx)
	if err != nil {
		return err
	}

	newIdx := make([]string, 0, len(idx))
	for _, id := range idx {
		if id != serverID {
			newIdx = append(newIdx, id)
		}
	}

	return d.saveServerIndex(ctx, newIdx)
}

func (d *DaprDB) ListServerConfigs(ctx context.Context) ([]*ServerConfig, error) {
	serverIDs, err := d.getServerIndex(ctx)
	if err != nil {
		return nil, err
	}
	if len(serverIDs) == 0 {
		return []*ServerConfig{}, nil
	}

	keys := make([]string, len(serverIDs))
	for i, id := range serverIDs {
		keys[i] = fmt.Sprintf("serverconfig:%s", id)
	}

	items, err := d.client.GetBulk(ctx, d.stateStoreName, keys)
	if err != nil {
		return nil, err
	}

	configs := make([]*ServerConfig, 0, len(items))
	for _, item := range items {
		if item.Error != "" {
			log.Printf("DaprDB.ListServerConfigs: error for key %q: %s", item.Key, item.Error)
			continue
		}
		if item.Value == nil {
			continue
		}
		var config ServerConfig
		if err := json.Unmarshal(item.Value, &config); err != nil {
			log.Printf("DaprDB.ListServerConfigs: error unmarshaling %q: %v", item.Key, err)
			continue
		}
		config.ID = keySuffix(item.Key)
		configs = append(configs, &config)
	}
	return configs, nil
}

func (d *DaprDB) SaveConfigSnapshot(ctx context.Context, serverID string, config *ServerConfig) error {
	key := fmt.Sprintf("configsnapshot:%s", serverID)
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return d.client.Save(ctx, d.stateStoreName, key, data)
}

func (d *DaprDB) GetConfigSnapshot(ctx context.Context, serverID string) (*ServerConfig, error) {
	key := fmt.Sprintf("configsnapshot:%s", serverID)
	item, err := d.client.Get(ctx, d.stateStoreName, key)
	if err != nil {
		return nil, err
	}
	if item == nil || item.Value == nil {
		return nil, fmt.Errorf("%w: config snapshot not found for %q", ErrNotFound, serverID)
	}
	var config ServerConfig
	if err := json.Unmarshal(item.Value, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (d *DaprDB) DeleteConfigSnapshot(ctx context.Context, serverID string) error {
	key := fmt.Sprintf("configsnapshot:%s", serverID)
	return d.client.Delete(ctx, d.stateStoreName, key)
}

func (d *DaprDB) GetPulumiSettings(ctx context.Context) (*PulumiSettings, error) {
	item, err := d.client.Get(ctx, d.stateStoreName, "pulumisettings")
	if err != nil {
		return nil, err
	}
	if item == nil || item.Value == nil {
		return nil, nil
	}
	var settings PulumiSettings
	if err := json.Unmarshal(item.Value, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func (d *DaprDB) SetPulumiSettings(ctx context.Context, settings *PulumiSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return d.client.Save(ctx, d.stateStoreName, "pulumisettings", data)
}

func (d *DaprDB) ListAllServerIDs(ctx context.Context) ([]string, error) {
	return d.getServerIndex(ctx)
}
