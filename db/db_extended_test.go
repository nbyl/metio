package db

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Whitelist Tests ---

func TestFirestoreDB_GetWhitelistConfig_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "instances").Return(mockCol)
	mockCol.On("Doc", "srv").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "whitelist").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnap, nil)
	mockSnap.On("DataTo", mock.AnythingOfType("*db.WhitelistConfig")).Return(nil).Run(func(args mock.Arguments) {
		cfg := args.Get(0).(*WhitelistConfig)
		cfg.Enabled = true
	})

	config, err := db.GetWhitelistConfig(ctx, "srv")
	assert.NoError(t, err)
	assert.True(t, config.Enabled)
}

func TestFirestoreDB_GetWhitelistConfig_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	_, err := db.GetWhitelistConfig(context.Background(), "srv")
	assert.Error(t, err)
}

func TestFirestoreDB_GetWhitelistConfig_GetError(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "instances").Return(mockCol)
	mockCol.On("Doc", "srv").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "whitelist").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(nil, assert.AnError)

	_, err := db.GetWhitelistConfig(ctx, "srv")
	assert.Error(t, err)
}

func TestFirestoreDB_SetWhitelistConfig_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	cfg := WhitelistConfig{Enabled: true}

	mockClient.On("Collection", "instances").Return(mockCol)
	mockCol.On("Doc", "srv").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "whitelist").Return(mockSubDoc)
	mockSubDoc.On("Set", ctx, cfg, mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.SetWhitelistConfig(ctx, "srv", cfg)
	assert.NoError(t, err)
}

func TestFirestoreDB_SetWhitelistConfig_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.SetWhitelistConfig(context.Background(), "srv", WhitelistConfig{})
	assert.Error(t, err)
}

func TestFirestoreDB_GetWhitelistEntries_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockIter := new(MockDocumentIterator)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "instances").Return(mockCol)
	mockCol.On("Doc", "srv").Return(mockDoc)
	mockDoc.On("Collection", "whitelist").Return(mockSubCol)
	mockSubCol.On("Documents", ctx).Return(mockIter)

	// Return one entry then done
	mockIter.On("Next").Return(mockSnap, nil).Once()
	mockIter.On("Next").Return(nil, assert.AnError).Once()
	mockIter.On("Stop").Return()
	mockSnap.On("DataTo", mock.AnythingOfType("*db.WhitelistEntry")).Return(nil).Run(func(args mock.Arguments) {
		e := args.Get(0).(*WhitelistEntry)
		e.Username = "Steve"
		e.UUID = "uuid-1"
	})

	entries, err := db.GetWhitelistEntries(ctx, "srv")
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "Steve", entries[0].Username)
}

func TestFirestoreDB_GetWhitelistEntries_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	_, err := db.GetWhitelistEntries(context.Background(), "srv")
	assert.Error(t, err)
}

func TestFirestoreDB_AddWhitelistEntry_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	entry := WhitelistEntry{Username: "Steve", UUID: "uuid-1", AddedAt: time.Now(), AddedBy: "admin"}

	mockClient.On("Collection", "instances").Return(mockCol)
	mockCol.On("Doc", "srv").Return(mockDoc)
	mockDoc.On("Collection", "whitelist").Return(mockSubCol)
	mockSubCol.On("Doc", "uuid-1").Return(mockSubDoc)
	mockSubDoc.On("Set", ctx, entry, mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.AddWhitelistEntry(ctx, "srv", entry)
	assert.NoError(t, err)
}

func TestFirestoreDB_AddWhitelistEntry_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.AddWhitelistEntry(context.Background(), "srv", WhitelistEntry{})
	assert.Error(t, err)
}

func TestFirestoreDB_RemoveWhitelistEntry_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "instances").Return(mockCol)
	mockCol.On("Doc", "srv").Return(mockDoc)
	mockDoc.On("Collection", "whitelist").Return(mockSubCol)
	mockSubCol.On("Doc", "uuid-1").Return(mockSubDoc)
	mockSubDoc.On("Delete", ctx).Return(&firestore.WriteResult{}, nil)

	err := db.RemoveWhitelistEntry(ctx, "srv", "uuid-1")
	assert.NoError(t, err)
}

func TestFirestoreDB_RemoveWhitelistEntry_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.RemoveWhitelistEntry(context.Background(), "srv", "uuid-1")
	assert.Error(t, err)
}

func TestFirestoreDB_SetWhitelistEntries_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.SetWhitelistEntries(context.Background(), "srv", nil)
	assert.Error(t, err)
}

// --- Provisioning Tests ---

func TestFirestoreDB_GetProvisioningStatus_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "provisioning").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnap, nil)
	mockSnap.On("DataTo", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil).Run(func(args mock.Arguments) {
		s := args.Get(0).(*ProvisioningStatus)
		s.State = ProvisioningStateInProgress
		s.CurrentStep = "creating_vm"
	})

	status, err := db.GetProvisioningStatus(ctx, "srv1")
	assert.NoError(t, err)
	assert.Equal(t, ProvisioningStateInProgress, status.State)
	assert.Equal(t, "creating_vm", status.CurrentStep)
}

func TestFirestoreDB_GetProvisioningStatus_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	_, err := db.GetProvisioningStatus(context.Background(), "srv1")
	assert.Error(t, err)
}

func TestFirestoreDB_GetProvisioningStatus_GetError(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "provisioning").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(nil, assert.AnError)

	_, err := db.GetProvisioningStatus(ctx, "srv1")
	assert.Error(t, err)
}

func TestFirestoreDB_GetProvisioningStatus_DataToError(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "provisioning").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnap, nil)
	mockSnap.On("DataTo", mock.AnythingOfType("*db.ProvisioningStatus")).Return(assert.AnError)

	_, err := db.GetProvisioningStatus(ctx, "srv1")
	assert.Error(t, err)
}

func TestFirestoreDB_UpdateProvisioningStatus_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	status := &ProvisioningStatus{State: ProvisioningStateCompleted}

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "provisioning").Return(mockSubDoc)
	mockSubDoc.On("Set", ctx, status, mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.UpdateProvisioningStatus(ctx, "srv1", status)
	assert.NoError(t, err)
}

func TestFirestoreDB_UpdateProvisioningStatus_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.UpdateProvisioningStatus(context.Background(), "srv1", &ProvisioningStatus{})
	assert.Error(t, err)
}

func TestFirestoreDB_AddProvisioningStep_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	step := ProvisioningStep{Name: "create_vm", Status: ProvisioningStateInProgress}

	// GetProvisioningStatus call
	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "provisioning").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnap, nil)
	mockSnap.On("DataTo", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil).Run(func(args mock.Arguments) {
		s := args.Get(0).(*ProvisioningStatus)
		s.State = ProvisioningStateInProgress
		s.Steps = []ProvisioningStep{}
	})

	// UpdateProvisioningStatus call
	mockSubDoc.On("Set", ctx, mock.AnythingOfType("*db.ProvisioningStatus"), mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.AddProvisioningStep(ctx, "srv1", step)
	assert.NoError(t, err)
}

func TestFirestoreDB_AddProvisioningStep_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.AddProvisioningStep(context.Background(), "srv1", ProvisioningStep{})
	assert.Error(t, err)
}

func TestFirestoreDB_CompleteProvisioning_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "provisioning").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnap, nil)
	mockSnap.On("DataTo", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil).Run(func(args mock.Arguments) {
		s := args.Get(0).(*ProvisioningStatus)
		s.State = ProvisioningStateInProgress
	})
	mockSubDoc.On("Set", ctx, mock.AnythingOfType("*db.ProvisioningStatus"), mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.CompleteProvisioning(ctx, "srv1", map[string]string{"ip": "1.2.3.4"})
	assert.NoError(t, err)
}

func TestFirestoreDB_CompleteProvisioning_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.CompleteProvisioning(context.Background(), "srv1", nil)
	assert.Error(t, err)
}

func TestFirestoreDB_FailProvisioning_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "provisioning").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnap, nil)
	mockSnap.On("DataTo", mock.AnythingOfType("*db.ProvisioningStatus")).Return(nil).Run(func(args mock.Arguments) {
		s := args.Get(0).(*ProvisioningStatus)
		s.State = ProvisioningStateInProgress
	})
	mockSubDoc.On("Set", ctx, mock.AnythingOfType("*db.ProvisioningStatus"), mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.FailProvisioning(ctx, "srv1", "disk full")
	assert.NoError(t, err)
}

func TestFirestoreDB_FailProvisioning_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.FailProvisioning(context.Background(), "srv1", "err")
	assert.Error(t, err)
}

// --- ServerConfig Tests ---

func TestFirestoreDB_CreateServerConfig_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	cfg := &ServerConfig{Name: "my-server"}

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	// Marker doc
	mockDoc.On("Set", ctx, mock.AnythingOfType("map[string]interface {}"), mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)
	// Config doc
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "config").Return(mockSubDoc)
	mockSubDoc.On("Set", ctx, cfg, mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.CreateServerConfig(ctx, "srv1", cfg)
	assert.NoError(t, err)
}

func TestFirestoreDB_CreateServerConfig_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.CreateServerConfig(context.Background(), "srv1", &ServerConfig{})
	assert.Error(t, err)
}

func TestFirestoreDB_GetServerConfig_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "config").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnap, nil)
	mockSnap.On("DataTo", mock.AnythingOfType("*db.ServerConfig")).Return(nil).Run(func(args mock.Arguments) {
		c := args.Get(0).(*ServerConfig)
		c.Name = "my-server"
	})

	cfg, err := db.GetServerConfig(ctx, "srv1")
	assert.NoError(t, err)
	assert.Equal(t, "my-server", cfg.Name)
}

func TestFirestoreDB_GetServerConfig_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	_, err := db.GetServerConfig(context.Background(), "srv1")
	assert.Error(t, err)
}

func TestFirestoreDB_GetServerConfig_GetError(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "config").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(nil, assert.AnError)

	_, err := db.GetServerConfig(ctx, "srv1")
	assert.Error(t, err)
}

func TestFirestoreDB_UpdateServerConfig_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	cfg := &ServerConfig{Name: "updated"}

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "config").Return(mockSubDoc)
	mockSubDoc.On("Set", ctx, cfg, mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.UpdateServerConfig(ctx, "srv1", cfg)
	assert.NoError(t, err)
}

func TestFirestoreDB_UpdateServerConfig_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.UpdateServerConfig(context.Background(), "srv1", &ServerConfig{})
	assert.Error(t, err)
}

func TestFirestoreDB_DeleteServerConfig_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Doc", "srv1").Return(mockDoc)
	// Config delete
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "config").Return(mockSubDoc)
	mockSubDoc.On("Delete", ctx).Return(&firestore.WriteResult{}, nil)
	// Marker delete
	mockDoc.On("Delete", ctx).Return(&firestore.WriteResult{}, nil)

	err := db.DeleteServerConfig(ctx, "srv1")
	assert.NoError(t, err)
}

func TestFirestoreDB_DeleteServerConfig_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.DeleteServerConfig(context.Background(), "srv1")
	assert.Error(t, err)
}

func TestFirestoreDB_ListServerConfigs_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockIter := new(MockDocumentIterator)
	mockSnap := new(MockDocumentSnapshot)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockConfigSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Documents", ctx).Return(mockIter)
	mockIter.On("Next").Return(mockSnap, nil).Once()
	mockIter.On("Next").Return(nil, assert.AnError).Once()
	mockIter.On("Stop").Return()

	mockSnap.On("GetDocumentRef").Return(mockDoc)
	mockSnap.On("GetID").Return("srv1")
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "config").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockConfigSnap, nil)
	mockConfigSnap.On("DataTo", mock.AnythingOfType("*db.ServerConfig")).Return(nil).Run(func(args mock.Arguments) {
		c := args.Get(0).(*ServerConfig)
		c.Name = "my-server"
	})

	configs, err := db.ListServerConfigs(ctx)
	assert.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, "srv1", configs[0].ID)
	assert.Equal(t, "my-server", configs[0].Name)
}

func TestFirestoreDB_ListServerConfigs_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	_, err := db.ListServerConfigs(context.Background())
	assert.Error(t, err)
}

func TestFirestoreDB_SetWhitelistEntries_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockIter := new(MockDocumentIterator)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	entries := []WhitelistEntry{
		{Username: "Steve", UUID: "uuid-1", AddedAt: time.Now(), AddedBy: "admin"},
	}

	mockClient.On("Collection", "instances").Return(mockCol)
	mockCol.On("Doc", "srv").Return(mockDoc)
	mockDoc.On("Collection", "whitelist").Return(mockSubCol)
	// Delete phase - iterator returns done immediately
	mockSubCol.On("Documents", ctx).Return(mockIter)
	mockIter.On("Next").Return(nil, assert.AnError).Once()
	mockIter.On("Stop").Return()
	// Add phase
	mockSubCol.On("Doc", "uuid-1").Return(mockSubDoc)
	mockSubDoc.On("Set", ctx, entries[0], mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.SetWhitelistEntries(ctx, "srv", entries)
	assert.NoError(t, err)
}

func TestFirestoreDB_GetWhitelistEntries_DataToError(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockIter := new(MockDocumentIterator)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "instances").Return(mockCol)
	mockCol.On("Doc", "srv").Return(mockDoc)
	mockDoc.On("Collection", "whitelist").Return(mockSubCol)
	mockSubCol.On("Documents", ctx).Return(mockIter)
	// Return one entry that fails DataTo, then done
	mockIter.On("Next").Return(mockSnap, nil).Once()
	mockIter.On("Next").Return(nil, assert.AnError).Once()
	mockIter.On("Stop").Return()
	mockSnap.On("DataTo", mock.AnythingOfType("*db.WhitelistEntry")).Return(assert.AnError)

	entries, err := db.GetWhitelistEntries(ctx, "srv")
	assert.NoError(t, err)
	assert.Len(t, entries, 0) // entry skipped due to DataTo error
}

func TestFirestoreDB_ListServerConfigs_ConfigGetError(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCol := new(MockCollectionRef)
	mockIter := new(MockDocumentIterator)
	mockSnap := new(MockDocumentSnapshot)
	mockDoc := new(MockDocumentRef)
	mockSubCol := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "servers").Return(mockCol)
	mockCol.On("Documents", ctx).Return(mockIter)
	mockIter.On("Next").Return(mockSnap, nil).Once()
	mockIter.On("Next").Return(nil, assert.AnError).Once()
	mockIter.On("Stop").Return()

	mockSnap.On("GetDocumentRef").Return(mockDoc)
	mockSnap.On("GetID").Return("srv1")
	mockDoc.On("Collection", "data").Return(mockSubCol)
	mockSubCol.On("Doc", "config").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(nil, assert.AnError)

	configs, err := db.ListServerConfigs(ctx)
	assert.NoError(t, err)
	assert.Len(t, configs, 0) // skipped due to Get error
}

// --- Provisioning State/Operation String Tests ---

func TestProvisioningState_String(t *testing.T) {
	assert.Equal(t, "PENDING", ProvisioningStatePending.String())
	assert.Equal(t, "IN_PROGRESS", ProvisioningStateInProgress.String())
	assert.Equal(t, "COMPLETED", ProvisioningStateCompleted.String())
	assert.Equal(t, "FAILED", ProvisioningStateFailed.String())
	assert.Equal(t, "UNKNOWN", ProvisioningState(99).String())
}

func TestProvisioningState_FirestoreValue(t *testing.T) {
	assert.Equal(t, "pending", ProvisioningStatePending.FirestoreValue())
	assert.Equal(t, "in_progress", ProvisioningStateInProgress.FirestoreValue())
	assert.Equal(t, "completed", ProvisioningStateCompleted.FirestoreValue())
	assert.Equal(t, "failed", ProvisioningStateFailed.FirestoreValue())
	assert.Equal(t, "unknown", ProvisioningState(99).FirestoreValue())
}

func TestProvisioningOperation_String(t *testing.T) {
	assert.Equal(t, "CREATE", ProvisioningOperationCreate.String())
	assert.Equal(t, "UPDATE", ProvisioningOperationUpdate.String())
	assert.Equal(t, "DESTROY", ProvisioningOperationDestroy.String())
	assert.Equal(t, "UNKNOWN", ProvisioningOperation(99).String())
}

func TestProvisioningOperation_FirestoreValue(t *testing.T) {
	assert.Equal(t, "create", ProvisioningOperationCreate.FirestoreValue())
	assert.Equal(t, "update", ProvisioningOperationUpdate.FirestoreValue())
	assert.Equal(t, "destroy", ProvisioningOperationDestroy.FirestoreValue())
	assert.Equal(t, "unknown", ProvisioningOperation(99).FirestoreValue())
}

// --- ServerState Tests ---

func TestServerState_String(t *testing.T) {
	assert.Equal(t, "STOPPED", ServerStateStopped.String())
	assert.Equal(t, "RUNNING", ServerStateRunning.String())
}

func TestServerState_IsRunning(t *testing.T) {
	assert.True(t, ServerStateRunning.IsRunning())
	assert.False(t, ServerStateStopped.IsRunning())
}

func TestServerState_IsStopped(t *testing.T) {
	assert.True(t, ServerStateStopped.IsStopped())
	assert.False(t, ServerStateRunning.IsStopped())
}

func TestServerState_IsTransitioning(t *testing.T) {
	assert.True(t, ServerStateStarting.IsTransitioning())
	assert.True(t, ServerStateStopping.IsTransitioning())
	assert.False(t, ServerStateRunning.IsTransitioning())
	assert.False(t, ServerStateStopped.IsTransitioning())
}
