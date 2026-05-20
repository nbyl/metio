package db

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetPulumiSettings_NotFound(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "settings").Return(mockCollection)
	mockCollection.On("Doc", "pulumi").Return(mockDoc)
	mockDoc.On("Get", ctx).Return(nil, status.Error(codes.NotFound, "document not found"))

	settings, err := db.GetPulumiSettings(ctx)

	assert.NoError(t, err)
	assert.Nil(t, settings)
	mockClient.AssertExpectations(t)
}

func TestGetPulumiSettings_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSnap := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	expected := &PulumiSettings{
		StateBucket:   "dev-metio-pulumi-state",
		Initialized:   true,
		InitializedAt: time.Now(),
		InitializedBy: "controller",
	}

	mockClient.On("Collection", "settings").Return(mockCollection)
	mockCollection.On("Doc", "pulumi").Return(mockDoc)
	mockDoc.On("Get", ctx).Return(mockSnap, nil)
	mockSnap.On("DataTo", mock.AnythingOfType("*db.PulumiSettings")).Run(func(args mock.Arguments) {
		dst := args.Get(0).(*PulumiSettings)
		*dst = *expected
	}).Return(nil)

	settings, err := db.GetPulumiSettings(ctx)

	assert.NoError(t, err)
	assert.Equal(t, expected.StateBucket, settings.StateBucket)
	assert.True(t, settings.Initialized)
	mockClient.AssertExpectations(t)
}

func TestGetPulumiSettings_Error(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()

	mockClient.On("Collection", "settings").Return(mockCollection)
	mockCollection.On("Doc", "pulumi").Return(mockDoc)
	mockDoc.On("Get", ctx).Return(nil, assert.AnError)

	_, err := db.GetPulumiSettings(ctx)

	assert.Error(t, err)
	mockClient.AssertExpectations(t)
}

func TestGetPulumiSettings_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	_, err := db.GetPulumiSettings(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "client is nil", err.Error())
}

func TestSetPulumiSettings_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	settings := &PulumiSettings{
		StateBucket:   "dev-metio-pulumi-state",
		Initialized:   true,
		InitializedAt: time.Now(),
		InitializedBy: "controller",
	}

	mockClient.On("Collection", "settings").Return(mockCollection)
	mockCollection.On("Doc", "pulumi").Return(mockDoc)
	mockDoc.On("Set", ctx, settings, mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.SetPulumiSettings(ctx, settings)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestSetPulumiSettings_Error(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	settings := &PulumiSettings{StateBucket: "test"}

	mockClient.On("Collection", "settings").Return(mockCollection)
	mockCollection.On("Doc", "pulumi").Return(mockDoc)
	mockDoc.On("Set", ctx, settings, mock.AnythingOfType("[]firestore.SetOption")).Return(nil, assert.AnError)

	err := db.SetPulumiSettings(ctx, settings)

	assert.Error(t, err)
	mockClient.AssertExpectations(t)
}

func TestSetPulumiSettings_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	err := db.SetPulumiSettings(context.Background(), &PulumiSettings{})
	assert.Error(t, err)
	assert.Equal(t, "client is nil", err.Error())
}
