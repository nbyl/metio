package db

import (
	"context"
	"errors"
	"log"
	"time"

	"cloud.google.com/go/firestore"
)

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
}

type FirestoreDB struct {
	client FirestoreClient
}

// FirestoreClient interface to allow mocking
type FirestoreClient interface {
	Collection(name string) CollectionRef
}

type CollectionRef interface {
	Doc(id string) DocumentRef
	Documents(ctx context.Context) DocumentIterator
}

type DocumentRef interface {
	Collection(id string) CollectionRef
	Set(ctx context.Context, data interface{}, opts ...firestore.SetOption) (*firestore.WriteResult, error)
	Get(ctx context.Context) (DocumentSnapshot, error)
	Delete(ctx context.Context) (*firestore.WriteResult, error)
}

type DocumentSnapshot interface {
	DataTo(dst interface{}) error
	GetDocumentRef() DocumentRef
	GetID() string
}

type DocumentIterator interface {
	Next() (DocumentSnapshot, error)
	Stop()
}

// Adapter to make firestore.Client implement our interface
type FirestoreClientAdapter struct {
	client *firestore.Client
}

func (a *FirestoreClientAdapter) Collection(name string) CollectionRef {
	return &CollectionRefAdapter{ref: a.client.Collection(name)}
}

type CollectionRefAdapter struct {
	ref *firestore.CollectionRef
}

func (a *CollectionRefAdapter) Doc(id string) DocumentRef {
	return &DocumentRefAdapter{ref: a.ref.Doc(id)}
}

func (a *CollectionRefAdapter) Documents(ctx context.Context) DocumentIterator {
	return &DocumentIteratorAdapter{iter: a.ref.Documents(ctx)}
}

type DocumentRefAdapter struct {
	ref *firestore.DocumentRef
}

func (a *DocumentRefAdapter) Collection(id string) CollectionRef {
	return &CollectionRefAdapter{ref: a.ref.Collection(id)}
}

func (a *DocumentRefAdapter) Set(ctx context.Context, data interface{}, opts ...firestore.SetOption) (*firestore.WriteResult, error) {
	return a.ref.Set(ctx, data, opts...)
}

func (a *DocumentRefAdapter) Get(ctx context.Context) (DocumentSnapshot, error) {
	snap, err := a.ref.Get(ctx)
	if err != nil {
		return nil, err
	}
	return &DocumentSnapshotAdapter{snap: snap}, nil
}

func (a *DocumentRefAdapter) Delete(ctx context.Context) (*firestore.WriteResult, error) {
	return a.ref.Delete(ctx)
}

type DocumentSnapshotAdapter struct {
	snap *firestore.DocumentSnapshot
}

func (a *DocumentSnapshotAdapter) DataTo(dst interface{}) error {
	return a.snap.DataTo(dst)
}

func (a *DocumentSnapshotAdapter) GetDocumentRef() DocumentRef {
	return &DocumentRefAdapter{ref: a.snap.Ref}
}

func (a *DocumentSnapshotAdapter) GetID() string {
	return a.snap.Ref.ID
}

type DocumentIteratorAdapter struct {
	iter *firestore.DocumentIterator
}

func (a *DocumentIteratorAdapter) Next() (DocumentSnapshot, error) {
	snap, err := a.iter.Next()
	if err != nil {
		return nil, err
	}
	return &DocumentSnapshotAdapter{snap: snap}, nil
}

func (a *DocumentIteratorAdapter) Stop() {
	a.iter.Stop()
}

func NewConnection(ctx context.Context, projectID, databaseID string) (DB, error) {
	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		return nil, err
	}
	adapter := &FirestoreClientAdapter{client: client}
	return &FirestoreDB{client: adapter}, nil
}

func (db *FirestoreDB) UpdateStatus(ctx context.Context, instanceName string, status Status) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	_, err := db.client.Collection("instances").Doc(instanceName).Collection("data").Doc("status").Set(ctx, status)
	if err != nil {
		return err
	}
	log.Printf("Successfully updated status for instance %s", instanceName)
	return nil
}

func (db *FirestoreDB) GetStatus(ctx context.Context, instanceName string) (Status, error) {
	if db.client == nil {
		return Status{}, errors.New("client is nil")
	}
	doc, err := db.client.Collection("instances").Doc(instanceName).Collection("data").Doc("status").Get(ctx)
	if err != nil {
		return Status{}, err
	}
	var status Status
	err = doc.DataTo(&status)
	return status, err
}

func (db *FirestoreDB) GetWhitelistConfig(ctx context.Context, instanceName string) (WhitelistConfig, error) {
	if db.client == nil {
		return WhitelistConfig{}, errors.New("client is nil")
	}
	doc, err := db.client.Collection("instances").Doc(instanceName).Collection("data").Doc("whitelist").Get(ctx)
	if err != nil {
		return WhitelistConfig{}, err
	}
	var config WhitelistConfig
	err = doc.DataTo(&config)
	return config, err
}

func (db *FirestoreDB) SetWhitelistConfig(ctx context.Context, instanceName string, config WhitelistConfig) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	_, err := db.client.Collection("instances").Doc(instanceName).Collection("data").Doc("whitelist").Set(ctx, config)
	if err != nil {
		return err
	}
	log.Printf("Successfully updated whitelist config for instance %s", instanceName)
	return nil
}

func (db *FirestoreDB) GetWhitelistEntries(ctx context.Context, instanceName string) ([]WhitelistEntry, error) {
	if db.client == nil {
		return nil, errors.New("client is nil")
	}
	iter := db.client.Collection("instances").Doc(instanceName).Collection("whitelist").Documents(ctx)
	defer iter.Stop()

	var entries []WhitelistEntry
	for {
		doc, err := iter.Next()
		if err != nil {
			// Check for iterator done - need to import google.golang.org/api/iterator
			break
		}
		var entry WhitelistEntry
		if err := doc.DataTo(&entry); err != nil {
			log.Printf("Error parsing whitelist entry: %v", err)
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (db *FirestoreDB) AddWhitelistEntry(ctx context.Context, instanceName string, entry WhitelistEntry) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	// Use UUID as document ID for reliable identification
	_, err := db.client.Collection("instances").Doc(instanceName).Collection("whitelist").Doc(entry.UUID).Set(ctx, entry)
	if err != nil {
		return err
	}
	log.Printf("Successfully added %s to whitelist for instance %s", entry.Username, instanceName)
	return nil
}

func (db *FirestoreDB) RemoveWhitelistEntry(ctx context.Context, instanceName string, uuid string) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	_, err := db.client.Collection("instances").Doc(instanceName).Collection("whitelist").Doc(uuid).Delete(ctx)
	if err != nil {
		return err
	}
	log.Printf("Successfully removed player with UUID %s from whitelist for instance %s", uuid, instanceName)
	return nil
}

func (db *FirestoreDB) SetWhitelistEntries(ctx context.Context, instanceName string, entries []WhitelistEntry) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	// First, delete all existing entries
	iter := db.client.Collection("instances").Doc(instanceName).Collection("whitelist").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		// Get the document reference from the snapshot - we need to extract UUID
		var entry WhitelistEntry
		if err := doc.DataTo(&entry); err == nil {
			db.client.Collection("instances").Doc(instanceName).Collection("whitelist").Doc(entry.UUID).Delete(ctx)
		}
	}
	iter.Stop()

	// Then add all new entries
	for _, entry := range entries {
		if err := db.AddWhitelistEntry(ctx, instanceName, entry); err != nil {
			return err
		}
	}
	log.Printf("Successfully set %d whitelist entries for instance %s", len(entries), instanceName)
	return nil
}

func (db *FirestoreDB) GetProvisioningStatus(ctx context.Context, serverID string) (*ProvisioningStatus, error) {
	if db.client == nil {
		return nil, errors.New("client is nil")
	}
	doc, err := db.client.Collection("servers").Doc(serverID).Collection("data").Doc("provisioning").Get(ctx)
	if err != nil {
		return nil, err
	}
	var status ProvisioningStatus
	err = doc.DataTo(&status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (db *FirestoreDB) UpdateProvisioningStatus(ctx context.Context, serverID string, status *ProvisioningStatus) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	_, err := db.client.Collection("servers").Doc(serverID).Collection("data").Doc("provisioning").Set(ctx, status)
	if err != nil {
		return err
	}
	return nil
}

func (db *FirestoreDB) AddProvisioningStep(ctx context.Context, serverID string, step ProvisioningStep) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	status, err := db.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		return err
	}
	status.Steps = append(status.Steps, step)
	status.CurrentStep = step.Name
	return db.UpdateProvisioningStatus(ctx, serverID, status)
}

func (db *FirestoreDB) CompleteProvisioning(ctx context.Context, serverID string, outputs map[string]string) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	status, err := db.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		return err
	}
	now := time.Now()
	status.State = ProvisioningStateCompleted
	status.CompletedAt = &now
	status.Outputs = outputs
	return db.UpdateProvisioningStatus(ctx, serverID, status)
}

func (db *FirestoreDB) FailProvisioning(ctx context.Context, serverID string, errMsg string) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	status, err := db.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		return err
	}
	now := time.Now()
	status.State = ProvisioningStateFailed
	status.CompletedAt = &now
	status.Error = errMsg
	return db.UpdateProvisioningStatus(ctx, serverID, status)
}

func (db *FirestoreDB) CreateServerConfig(ctx context.Context, serverID string, config *ServerConfig) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	// Write a marker document at servers/{id} so ListServerConfigs can discover it
	_, err := db.client.Collection("servers").Doc(serverID).Set(ctx, map[string]interface{}{
		"createdAt": time.Now(),
	})
	if err != nil {
		return err
	}
	_, err = db.client.Collection("servers").Doc(serverID).Collection("data").Doc("config").Set(ctx, config)
	if err != nil {
		return err
	}
	return nil
}

func (db *FirestoreDB) GetServerConfig(ctx context.Context, serverID string) (*ServerConfig, error) {
	if db.client == nil {
		return nil, errors.New("client is nil")
	}
	doc, err := db.client.Collection("servers").Doc(serverID).Collection("data").Doc("config").Get(ctx)
	if err != nil {
		return nil, err
	}
	var config ServerConfig
	err = doc.DataTo(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (db *FirestoreDB) UpdateServerConfig(ctx context.Context, serverID string, config *ServerConfig) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	_, err := db.client.Collection("servers").Doc(serverID).Collection("data").Doc("config").Set(ctx, config)
	if err != nil {
		return err
	}
	return nil
}

func (db *FirestoreDB) DeleteServerConfig(ctx context.Context, serverID string) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	_, err := db.client.Collection("servers").Doc(serverID).Collection("data").Doc("config").Delete(ctx)
	if err != nil {
		return err
	}
	// Delete the marker document at servers/{id}
	_, err = db.client.Collection("servers").Doc(serverID).Delete(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (db *FirestoreDB) ListServerConfigs(ctx context.Context) ([]*ServerConfig, error) {
	if db.client == nil {
		return nil, errors.New("client is nil")
	}
	iter := db.client.Collection("servers").Documents(ctx)
	defer iter.Stop()

	var configs []*ServerConfig
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		configDoc, err := doc.GetDocumentRef().Collection("data").Doc("config").Get(ctx)
		if err != nil {
			continue
		}
		var config ServerConfig
		if err := configDoc.DataTo(&config); err != nil {
			log.Printf("Error parsing server config: %v", err)
			continue
		}
		config.ID = doc.GetID()
		configs = append(configs, &config)
	}
	return configs, nil
}

func (db *FirestoreDB) SaveConfigSnapshot(ctx context.Context, serverID string, config *ServerConfig) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	_, err := db.client.Collection("servers").Doc(serverID).Collection("data").Doc("config_previous").Set(ctx, config)
	return err
}

func (db *FirestoreDB) GetConfigSnapshot(ctx context.Context, serverID string) (*ServerConfig, error) {
	if db.client == nil {
		return nil, errors.New("client is nil")
	}
	doc, err := db.client.Collection("servers").Doc(serverID).Collection("data").Doc("config_previous").Get(ctx)
	if err != nil {
		return nil, err
	}
	var config ServerConfig
	err = doc.DataTo(&config)
	return &config, err
}

func (db *FirestoreDB) DeleteConfigSnapshot(ctx context.Context, serverID string) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	_, err := db.client.Collection("servers").Doc(serverID).Collection("data").Doc("config_previous").Delete(ctx)
	return err
}
