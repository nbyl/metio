package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestDaprDB() (*DaprDB, *MockDaprClient) {
	mc := new(MockDaprClient)
	return &DaprDB{client: mc, stateStoreName: "statestore"}, mc
}

func TestDaprDB_UpdateStatus_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	status := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
		Uptime:    "2h30m",
	}
	expectedData, _ := json.Marshal(status)

	mc.On("Save", ctx, "statestore", "status:test-instance", expectedData).Return(nil)

	err := db.UpdateStatus(ctx, "test-instance", status)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_UpdateStatus_Error(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	status := Status{Players: Players{Current: 0, Max: 10}}

	mc.On("Save", ctx, "statestore", "status:test-instance", mock.Anything).Return(assert.AnError)

	err := db.UpdateStatus(ctx, "test-instance", status)
	assert.Error(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetStatus_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	expected := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: now,
		Uptime:    "2h30m",
	}
	data, _ := json.Marshal(expected)

	mc.On("Get", ctx, "statestore", "status:test-instance").Return(&daprStateItem{Key: "status:test-instance", Value: data}, nil)

	status, err := db.GetStatus(ctx, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, expected, status)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetStatus_NotFound(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "status:test-instance").Return(nil, nil)

	_, err := db.GetStatus(ctx, "test-instance")
	assert.ErrorIs(t, err, ErrNotFound)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetStatus_Error(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "status:test-instance").Return(nil, assert.AnError)

	_, err := db.GetStatus(ctx, "test-instance")
	assert.Error(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetWhitelistConfig_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	expected := WhitelistConfig{Enabled: true}
	data, _ := json.Marshal(expected)

	mc.On("Get", ctx, "statestore", "whitelistcfg:test-instance").Return(&daprStateItem{Value: data}, nil)

	config, err := db.GetWhitelistConfig(ctx, "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, expected, config)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetWhitelistConfig_NotFound(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "whitelistcfg:test-instance").Return(nil, nil)

	_, err := db.GetWhitelistConfig(ctx, "test-instance")
	assert.ErrorIs(t, err, ErrNotFound)
	mc.AssertExpectations(t)
}

func TestDaprDB_SetWhitelistConfig_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	config := WhitelistConfig{Enabled: false}
	data, _ := json.Marshal(config)

	mc.On("Save", ctx, "statestore", "whitelistcfg:test-instance", data).Return(nil)

	err := db.SetWhitelistConfig(ctx, "test-instance", config)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetWhitelistEntries_Empty(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "whitelistidx:test-instance").Return(nil, nil)

	entries, err := db.GetWhitelistEntries(ctx, "test-instance")
	assert.NoError(t, err)
	assert.Empty(t, entries)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetWhitelistEntries_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	now := time.Now()

	entry1 := WhitelistEntry{Username: "player1", UUID: "uuid-1", AddedAt: now, AddedBy: "admin"}
	entry2 := WhitelistEntry{Username: "player2", UUID: "uuid-2", AddedAt: now, AddedBy: "admin"}
	data1, _ := json.Marshal(entry1)
	data2, _ := json.Marshal(entry2)

	idxData, _ := json.Marshal(whitelistIndex{UUIDs: []string{"uuid-1", "uuid-2"}})

	mc.On("Get", ctx, "statestore", "whitelistidx:test-instance").Return(&daprStateItem{Value: idxData}, nil)
	mc.On("GetBulk", ctx, "statestore", []string{"whitelist:test-instance:uuid-1", "whitelist:test-instance:uuid-2"}).
		Return([]*daprBulkStateItem{
			{Key: "whitelist:test-instance:uuid-1", Value: data1},
			{Key: "whitelist:test-instance:uuid-2", Value: data2},
		}, nil)

	entries, err := db.GetWhitelistEntries(ctx, "test-instance")
	assert.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "player1", entries[0].Username)
	assert.Equal(t, "player2", entries[1].Username)
	mc.AssertExpectations(t)
}

func TestDaprDB_AddWhitelistEntry_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	entry := WhitelistEntry{Username: "player1", UUID: "uuid-1", AddedAt: time.Now(), AddedBy: "admin"}
	data, _ := json.Marshal(entry)

	mc.On("Save", ctx, "statestore", "whitelist:test-instance:uuid-1", data).Return(nil)
	mc.On("Get", ctx, "statestore", "whitelistidx:test-instance").Return(nil, nil)
	mc.On("Save", ctx, "statestore", "whitelistidx:test-instance", mock.Anything).Return(nil)

	err := db.AddWhitelistEntry(ctx, "test-instance", entry)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_AddWhitelistEntry_Duplicate(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	entry := WhitelistEntry{Username: "player1", UUID: "uuid-1", AddedAt: time.Now(), AddedBy: "admin"}
	data, _ := json.Marshal(entry)
	idxData, _ := json.Marshal(whitelistIndex{UUIDs: []string{"uuid-1"}})

	mc.On("Save", ctx, "statestore", "whitelist:test-instance:uuid-1", data).Return(nil)
	mc.On("Get", ctx, "statestore", "whitelistidx:test-instance").Return(&daprStateItem{Value: idxData}, nil)

	err := db.AddWhitelistEntry(ctx, "test-instance", entry)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_RemoveWhitelistEntry_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	idxData, _ := json.Marshal(whitelistIndex{UUIDs: []string{"uuid-1", "uuid-2"}})
	newIdxData, _ := json.Marshal(whitelistIndex{UUIDs: []string{"uuid-2"}})

	mc.On("Delete", ctx, "statestore", "whitelist:test-instance:uuid-1").Return(nil)
	mc.On("Get", ctx, "statestore", "whitelistidx:test-instance").Return(&daprStateItem{Value: idxData}, nil)
	mc.On("Save", ctx, "statestore", "whitelistidx:test-instance", newIdxData).Return(nil)

	err := db.RemoveWhitelistEntry(ctx, "test-instance", "uuid-1")
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_SetWhitelistEntries_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	now := time.Now()

	oldIdxData, _ := json.Marshal(whitelistIndex{UUIDs: []string{"old-uuid"}})

	entry1 := WhitelistEntry{Username: "player1", UUID: "new-uuid-1", AddedAt: now, AddedBy: "admin"}
	data1, _ := json.Marshal(entry1)
	newIdxData, _ := json.Marshal(whitelistIndex{UUIDs: []string{"new-uuid-1"}})

	mc.On("Get", ctx, "statestore", "whitelistidx:test-instance").Return(&daprStateItem{Value: oldIdxData}, nil)
	mc.On("Delete", ctx, "statestore", "whitelist:test-instance:old-uuid").Return(nil)
	mc.On("Save", ctx, "statestore", "whitelist:test-instance:new-uuid-1", data1).Return(nil)
	mc.On("Save", ctx, "statestore", "whitelistidx:test-instance", newIdxData).Return(nil)

	err := db.SetWhitelistEntries(ctx, "test-instance", []WhitelistEntry{entry1})
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetProvisioningStatus_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	expected := &ProvisioningStatus{
		ID:        "srv-1",
		Operation: ProvisioningOperationCreate,
		State:     ProvisioningStateInProgress,
	}
	data, _ := json.Marshal(expected)

	mc.On("Get", ctx, "statestore", "provisioning:srv-1").Return(&daprStateItem{Value: data}, nil)

	status, err := db.GetProvisioningStatus(ctx, "srv-1")
	assert.NoError(t, err)
	assert.Equal(t, expected.ID, status.ID)
	assert.Equal(t, ProvisioningOperationCreate, status.Operation)
	assert.Equal(t, ProvisioningStateInProgress, status.State)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetProvisioningStatus_NotFound(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "provisioning:srv-1").Return(nil, nil)

	_, err := db.GetProvisioningStatus(ctx, "srv-1")
	assert.ErrorIs(t, err, ErrNotFound)
	mc.AssertExpectations(t)
}

func TestDaprDB_UpdateProvisioningStatus_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	status := &ProvisioningStatus{
		ID:    "srv-1",
		State: ProvisioningStatePending,
	}
	data, _ := json.Marshal(status)

	mc.On("Save", ctx, "statestore", "provisioning:srv-1", data).Return(nil)

	err := db.UpdateProvisioningStatus(ctx, "srv-1", status)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_AddProvisioningStep_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	existing := &ProvisioningStatus{
		ID:    "srv-1",
		State: ProvisioningStateInProgress,
	}
	existingData, _ := json.Marshal(existing)
	step := ProvisioningStep{Name: "deploy", Status: ProvisioningStateInProgress}

	mc.On("Get", ctx, "statestore", "provisioning:srv-1").Return(&daprStateItem{Value: existingData}, nil)
	mc.On("Save", ctx, "statestore", "provisioning:srv-1", mock.Anything).Return(nil)

	err := db.AddProvisioningStep(ctx, "srv-1", step)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_CompleteProvisioning_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	existing := &ProvisioningStatus{
		ID:    "srv-1",
		State: ProvisioningStateInProgress,
	}
	existingData, _ := json.Marshal(existing)

	mc.On("Get", ctx, "statestore", "provisioning:srv-1").Return(&daprStateItem{Value: existingData}, nil)
	mc.On("Save", ctx, "statestore", "provisioning:srv-1", mock.Anything).Return(nil)

	err := db.CompleteProvisioning(ctx, "srv-1", map[string]string{"ip": "10.0.0.1"})
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_FailProvisioning_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	existing := &ProvisioningStatus{
		ID:    "srv-1",
		State: ProvisioningStateInProgress,
	}
	existingData, _ := json.Marshal(existing)

	mc.On("Get", ctx, "statestore", "provisioning:srv-1").Return(&daprStateItem{Value: existingData}, nil)
	mc.On("Save", ctx, "statestore", "provisioning:srv-1", mock.Anything).Return(nil)

	err := db.FailProvisioning(ctx, "srv-1", "something went wrong")
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetServerConfig_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	expected := &ServerConfig{Name: "test-server", Region: "us-central1"}
	data, _ := json.Marshal(expected)

	mc.On("Get", ctx, "statestore", "serverconfig:srv-1").Return(&daprStateItem{Value: data}, nil)

	config, err := db.GetServerConfig(ctx, "srv-1")
	assert.NoError(t, err)
	assert.Equal(t, "srv-1", config.ID)
	assert.Equal(t, "test-server", config.Name)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetServerConfig_NotFound(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "serverconfig:srv-1").Return(nil, nil)

	_, err := db.GetServerConfig(ctx, "srv-1")
	assert.ErrorIs(t, err, ErrNotFound)
	mc.AssertExpectations(t)
}

func TestDaprDB_CreateServerConfig_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	config := &ServerConfig{Name: "test-server"}
	data, _ := json.Marshal(config)
	newIdxData, _ := json.Marshal(serverIndex{ServerIDs: []string{"srv-1"}})

	mc.On("Save", ctx, "statestore", "serverconfig:srv-1", data).Return(nil)
	mc.On("Get", ctx, "statestore", "serverindex").Return(nil, nil)
	mc.On("Save", ctx, "statestore", "serverindex", newIdxData).Return(nil)

	err := db.CreateServerConfig(ctx, "srv-1", config)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_UpdateServerConfig_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	config := &ServerConfig{Name: "updated-server"}
	data, _ := json.Marshal(config)

	mc.On("Save", ctx, "statestore", "serverconfig:srv-1", data).Return(nil)

	err := db.UpdateServerConfig(ctx, "srv-1", config)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_DeleteServerConfig_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	idxData, _ := json.Marshal(serverIndex{ServerIDs: []string{"srv-1", "srv-2"}})
	newIdxData, _ := json.Marshal(serverIndex{ServerIDs: []string{"srv-2"}})

	mc.On("Delete", ctx, "statestore", "serverconfig:srv-1").Return(nil)
	mc.On("Get", ctx, "statestore", "serverindex").Return(&daprStateItem{Value: idxData}, nil)
	mc.On("Save", ctx, "statestore", "serverindex", newIdxData).Return(nil)

	err := db.DeleteServerConfig(ctx, "srv-1")
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_DeleteServerConfig_IndexNotFound(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Delete", ctx, "statestore", "serverconfig:srv-1").Return(nil)
	mc.On("Get", ctx, "statestore", "serverindex").Return(nil, nil)
	mc.On("Save", ctx, "statestore", "serverindex", []byte(`{"server_ids":[]}`)).Return(nil)

	err := db.DeleteServerConfig(ctx, "srv-1")
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_ListServerConfigs_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	idxData, _ := json.Marshal(serverIndex{ServerIDs: []string{"srv-1", "srv-2"}})

	config1 := ServerConfig{Name: "server-a"}
	config2 := ServerConfig{Name: "server-b"}
	data1, _ := json.Marshal(config1)
	data2, _ := json.Marshal(config2)

	mc.On("Get", ctx, "statestore", "serverindex").Return(&daprStateItem{Value: idxData}, nil)
	mc.On("GetBulk", ctx, "statestore", []string{"serverconfig:srv-1", "serverconfig:srv-2"}).
		Return([]*daprBulkStateItem{
			{Key: "serverconfig:srv-1", Value: data1},
			{Key: "serverconfig:srv-2", Value: data2},
		}, nil)

	configs, err := db.ListServerConfigs(ctx)
	assert.NoError(t, err)
	assert.Len(t, configs, 2)
	assert.Equal(t, "srv-1", configs[0].ID)
	assert.Equal(t, "server-a", configs[0].Name)
	assert.Equal(t, "srv-2", configs[1].ID)
	assert.Equal(t, "server-b", configs[1].Name)
	mc.AssertExpectations(t)
}

func TestDaprDB_ListServerConfigs_Empty(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "serverindex").Return(nil, nil)

	configs, err := db.ListServerConfigs(ctx)
	assert.NoError(t, err)
	assert.Empty(t, configs)
	mc.AssertExpectations(t)
}

func TestDaprDB_SaveConfigSnapshot_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	config := &ServerConfig{Name: "snapshot-config"}
	data, _ := json.Marshal(config)

	mc.On("Save", ctx, "statestore", "configsnapshot:srv-1", data).Return(nil)

	err := db.SaveConfigSnapshot(ctx, "srv-1", config)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetConfigSnapshot_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	expected := &ServerConfig{Name: "snapshot-config"}
	data, _ := json.Marshal(expected)

	mc.On("Get", ctx, "statestore", "configsnapshot:srv-1").Return(&daprStateItem{Value: data}, nil)

	config, err := db.GetConfigSnapshot(ctx, "srv-1")
	assert.NoError(t, err)
	assert.Equal(t, "snapshot-config", config.Name)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetConfigSnapshot_NotFound(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "configsnapshot:srv-1").Return(nil, nil)

	_, err := db.GetConfigSnapshot(ctx, "srv-1")
	assert.ErrorIs(t, err, ErrNotFound)
	mc.AssertExpectations(t)
}

func TestDaprDB_DeleteConfigSnapshot_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Delete", ctx, "statestore", "configsnapshot:srv-1").Return(nil)

	err := db.DeleteConfigSnapshot(ctx, "srv-1")
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetPulumiSettings_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	expected := &PulumiSettings{
		StateBucket: "bucket-1",
		Initialized: true,
	}
	data, _ := json.Marshal(expected)

	mc.On("Get", ctx, "statestore", "pulumisettings").Return(&daprStateItem{Value: data}, nil)

	settings, err := db.GetPulumiSettings(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "bucket-1", settings.StateBucket)
	assert.True(t, settings.Initialized)
	mc.AssertExpectations(t)
}

func TestDaprDB_GetPulumiSettings_NotFound(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "pulumisettings").Return(nil, nil)

	settings, err := db.GetPulumiSettings(ctx)
	assert.NoError(t, err)
	assert.Nil(t, settings)
	mc.AssertExpectations(t)
}

func TestDaprDB_SetPulumiSettings_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	settings := &PulumiSettings{StateBucket: "bucket-1"}
	data, _ := json.Marshal(settings)

	mc.On("Save", ctx, "statestore", "pulumisettings", data).Return(nil)

	err := db.SetPulumiSettings(ctx, settings)
	assert.NoError(t, err)
	mc.AssertExpectations(t)
}

func TestDaprDB_ListAllServerIDs_Success(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()
	idxData, _ := json.Marshal(serverIndex{ServerIDs: []string{"srv-1", "srv-2"}})

	mc.On("Get", ctx, "statestore", "serverindex").Return(&daprStateItem{Value: idxData}, nil)

	ids, err := db.ListAllServerIDs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"srv-1", "srv-2"}, ids)
	mc.AssertExpectations(t)
}

func TestDaprDB_ListAllServerIDs_Empty(t *testing.T) {
	db, mc := newTestDaprDB()
	ctx := context.Background()

	mc.On("Get", ctx, "statestore", "serverindex").Return(nil, nil)

	ids, err := db.ListAllServerIDs(ctx)
	assert.NoError(t, err)
	assert.Empty(t, ids)
	mc.AssertExpectations(t)
}

func TestDaprDB_DaprDBImplementsDB(t *testing.T) {
	var _ DB = &DaprDB{}
}
