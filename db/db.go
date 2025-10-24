package db

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
)

type DB interface {
	UpdateStatus(ctx context.Context, instanceName string, status Status) error
	GetStatus(ctx context.Context, instanceName string) (Status, error)
}

type FirestoreDB struct {
	client *firestore.Client
}

func NewConnection(ctx context.Context, projectID, databaseID string) (DB, error) {
	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		return nil, err
	}
	return &FirestoreDB{client: client}, nil
}

func (db *FirestoreDB) UpdateStatus(ctx context.Context, instanceName string, status Status) error {
	_, err := db.client.Collection("instances").Doc(instanceName).Collection("data").Doc("status").Set(ctx, status)
	if err != nil {
		return err
	}
	log.Printf("Successfully updated status for instance %s", instanceName)
	return nil
}

func (db *FirestoreDB) GetStatus(ctx context.Context, instanceName string) (Status, error) {
	doc, err := db.client.Collection("instances").Doc(instanceName).Collection("data").Doc("status").Get(ctx)
	if err != nil {
		return Status{}, err
	}
	var status Status
	err = doc.DataTo(&status)
	return status, err
}
