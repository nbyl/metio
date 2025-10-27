package db

import (
	"context"
	"errors"
	"log"

	"cloud.google.com/go/firestore"
)

type DB interface {
	UpdateStatus(ctx context.Context, instanceName string, status Status) error
	GetStatus(ctx context.Context, instanceName string) (Status, error)
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
}

type DocumentRef interface {
	Collection(id string) CollectionRef
	Set(ctx context.Context, data interface{}, opts ...firestore.SetOption) (*firestore.WriteResult, error)
	Get(ctx context.Context) (DocumentSnapshot, error)
}

type DocumentSnapshot interface {
	DataTo(dst interface{}) error
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

type DocumentSnapshotAdapter struct {
	snap *firestore.DocumentSnapshot
}

func (a *DocumentSnapshotAdapter) DataTo(dst interface{}) error {
	return a.snap.DataTo(dst)
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
