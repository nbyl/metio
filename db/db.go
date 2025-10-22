package db

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
)

type DB interface {
	UpdateStatus(ctx context.Context, instanceName string, status Status) error
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
