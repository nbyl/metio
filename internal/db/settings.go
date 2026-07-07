package db

import (
	"context"
	"errors"
	"time"

	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PulumiSettings stores the auto-provisioned Pulumi state bucket configuration.
type PulumiSettings struct {
	StateBucket   string    `json:"stateBucket" firestore:"stateBucket"`
	Initialized   bool      `json:"initialized" firestore:"initialized"`
	InitializedAt time.Time `json:"initializedAt" firestore:"initializedAt"`
	InitializedBy string    `json:"initializedBy" firestore:"initializedBy"`
}

func (db *FirestoreDB) GetPulumiSettings(ctx context.Context) (*PulumiSettings, error) {
	if db.client == nil {
		return nil, errors.New("client is nil")
	}
	doc, err := db.client.Collection("settings").Doc("pulumi").Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	var settings PulumiSettings
	err = doc.DataTo(&settings)
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (db *FirestoreDB) SetPulumiSettings(ctx context.Context, settings *PulumiSettings) error {
	if db.client == nil {
		return errors.New("client is nil")
	}
	_, err := db.client.Collection("settings").Doc("pulumi").Set(ctx, settings)
	if err != nil {
		return err
	}
	return nil
}

// ListAllServerIDs returns all server IDs from the servers collection.
func (db *FirestoreDB) ListAllServerIDs(ctx context.Context) ([]string, error) {
	if db.client == nil {
		return nil, errors.New("client is nil")
	}
	iter := db.client.Collection("servers").Documents(ctx)
	defer iter.Stop()

	var ids []string
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		ids = append(ids, doc.GetID())
	}
	return ids, nil
}
