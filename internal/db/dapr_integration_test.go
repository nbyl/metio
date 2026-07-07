//go:build integration

package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newIntegrationDaprDB(t *testing.T) (DB, string) {
	t.Helper()

	ctx := context.Background()
	d, err := NewDaprDB(ctx, "statestore")
	if err != nil {
		t.Skipf("Dapr sidecar not available (is 'make dev-up' running?): %v", err)
	}

	id := uuid.NewString()[:8]
	return d, id
}

func TestDaprDB_Integration_StatusCRUD(t *testing.T) {
	db, id := newIntegrationDaprDB(t)
	ctx := context.Background()
	instance := fmt.Sprintf("test-instance-%s", id)
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	status := Status{
		Players:   Players{Current: 3, Max: 20},
		Timestamp: now,
		Uptime:    "1h30m",
		Version:   "1.21.1",
	}

	err := db.UpdateStatus(ctx, instance, status)
	require.NoError(t, err)

	got, err := db.GetStatus(ctx, instance)
	require.NoError(t, err)
	assert.Equal(t, status.Players.Current, got.Players.Current)
	assert.Equal(t, status.Players.Max, got.Players.Max)
	assert.Equal(t, status.Uptime, got.Uptime)
	assert.Equal(t, status.Version, got.Version)

	status.Players.Current = 5
	err = db.UpdateStatus(ctx, instance, status)
	require.NoError(t, err)

	got, err = db.GetStatus(ctx, instance)
	require.NoError(t, err)
	assert.Equal(t, 5, got.Players.Current)
}

func TestDaprDB_Integration_WhitelistCRUD(t *testing.T) {
	db, id := newIntegrationDaprDB(t)
	ctx := context.Background()
	instance := fmt.Sprintf("test-whitelist-%s", id)

	config := WhitelistConfig{Enabled: true}
	err := db.SetWhitelistConfig(ctx, instance, config)
	require.NoError(t, err)

	gotConfig, err := db.GetWhitelistConfig(ctx, instance)
	require.NoError(t, err)
	assert.True(t, gotConfig.Enabled)

	entry1 := WhitelistEntry{
		Username:  "player1",
		UUID:      uuid.NewString(),
		AddedAt:   time.Now(),
		AddedBy:   "integration-test",
	}
	entry2 := WhitelistEntry{
		Username:  "player2",
		UUID:      uuid.NewString(),
		AddedAt:   time.Now(),
		AddedBy:   "integration-test",
	}

	err = db.AddWhitelistEntry(ctx, instance, entry1)
	require.NoError(t, err)

	err = db.AddWhitelistEntry(ctx, instance, entry2)
	require.NoError(t, err)

	entries, err := db.GetWhitelistEntries(ctx, instance)
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	err = db.RemoveWhitelistEntry(ctx, instance, entry1.UUID)
	require.NoError(t, err)

	entries, err = db.GetWhitelistEntries(ctx, instance)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "player2", entries[0].Username)

	err = db.SetWhitelistEntries(ctx, instance, []WhitelistEntry{entry1})
	require.NoError(t, err)

	entries, err = db.GetWhitelistEntries(ctx, instance)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "player1", entries[0].Username)
}

func TestDaprDB_Integration_ProvisioningReadModifyWrite(t *testing.T) {
	db, id := newIntegrationDaprDB(t)
	ctx := context.Background()
	serverID := fmt.Sprintf("test-provision-%s", id)

	status := &ProvisioningStatus{
		ID:        serverID,
		Operation: ProvisioningOperationCreate,
		State:     ProvisioningStatePending,
	}
	err := db.UpdateProvisioningStatus(ctx, serverID, status)
	require.NoError(t, err)

	got, err := db.GetProvisioningStatus(ctx, serverID)
	require.NoError(t, err)
	assert.Equal(t, ProvisioningOperationCreate, got.Operation)
	assert.Equal(t, ProvisioningStatePending, got.State)

	step := ProvisioningStep{
		Name:      "create-vm",
		Status:    ProvisioningStateInProgress,
		Timestamp: time.Now(),
	}
	err = db.AddProvisioningStep(ctx, serverID, step)
	require.NoError(t, err)

	err = db.CompleteProvisioning(ctx, serverID, map[string]string{"ip": "10.0.0.1"})
	require.NoError(t, err)

	got, err = db.GetProvisioningStatus(ctx, serverID)
	require.NoError(t, err)
	assert.Equal(t, ProvisioningStateCompleted, got.State)
	assert.Len(t, got.Steps, 1)

	status2 := &ProvisioningStatus{
		ID:    fmt.Sprintf("test-provision-%s-fail", id),
		State: ProvisioningStateInProgress,
	}
	err = db.UpdateProvisioningStatus(ctx, status2.ID, status2)
	require.NoError(t, err)

	err = db.FailProvisioning(ctx, status2.ID, "something went wrong")
	require.NoError(t, err)

	got2, err := db.GetProvisioningStatus(ctx, status2.ID)
	require.NoError(t, err)
	assert.Equal(t, ProvisioningStateFailed, got2.State)
	assert.Equal(t, "something went wrong", got2.Error)
}

func TestDaprDB_Integration_ServerConfigCRUD(t *testing.T) {
	db, id := newIntegrationDaprDB(t)
	ctx := context.Background()
	serverID := fmt.Sprintf("test-config-%s", id)

	config := &ServerConfig{
		Name:       "integration-test-server",
		Region:     "us-central1",
		Zone:       "us-central1-a",
		MachineType: "e2-small",
		DiskSizeGB: 20,
	}
	err := db.CreateServerConfig(ctx, serverID, config)
	require.NoError(t, err)

	got, err := db.GetServerConfig(ctx, serverID)
	require.NoError(t, err)
	assert.Equal(t, serverID, got.ID)
	assert.Equal(t, "integration-test-server", got.Name)
	assert.Equal(t, "us-central1", got.Region)

	config.Name = "updated-server"
	err = db.UpdateServerConfig(ctx, serverID, config)
	require.NoError(t, err)

	got, err = db.GetServerConfig(ctx, serverID)
	require.NoError(t, err)
	assert.Equal(t, "updated-server", got.Name)

	configs, err := db.ListServerConfigs(ctx)
	require.NoError(t, err)
	found := false
	for _, c := range configs {
		if c.ID == serverID {
			found = true
			break
		}
	}
	assert.True(t, found, "server config should appear in ListServerConfigs")

	err = db.DeleteServerConfig(ctx, serverID)
	require.NoError(t, err)

	_, err = db.GetServerConfig(ctx, serverID)
	assert.Error(t, err)
}

func TestDaprDB_Integration_ConfigSnapshot(t *testing.T) {
	db, id := newIntegrationDaprDB(t)
	ctx := context.Background()
	serverID := fmt.Sprintf("test-snapshot-%s", id)

	snapshot := &ServerConfig{
		Name:   "snapshot-test",
		Region: "europe-west3",
	}
	err := db.SaveConfigSnapshot(ctx, serverID, snapshot)
	require.NoError(t, err)

	got, err := db.GetConfigSnapshot(ctx, serverID)
	require.NoError(t, err)
	assert.Equal(t, "snapshot-test", got.Name)
	assert.Equal(t, "europe-west3", got.Region)

	err = db.DeleteConfigSnapshot(ctx, serverID)
	require.NoError(t, err)

	_, err = db.GetConfigSnapshot(ctx, serverID)
	assert.Error(t, err)
}

func TestDaprDB_Integration_PulumiSettings(t *testing.T) {
	db, id := newIntegrationDaprDB(t)
	ctx := context.Background()

	settings := &PulumiSettings{
		StateBucket: fmt.Sprintf("test-bucket-%s", id),
		Initialized: true,
	}
	err := db.SetPulumiSettings(ctx, settings)
	require.NoError(t, err)

	got, err := db.GetPulumiSettings(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, settings.StateBucket, got.StateBucket)
	assert.True(t, got.Initialized)
}
