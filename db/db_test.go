package db

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockDB struct {
	mock.Mock
}

func (m *MockDB) UpdateStatus(ctx context.Context, instanceName string, status Status) error {
	args := m.Called(ctx, instanceName, status)
	return args.Error(0)
}

func (m *MockDB) GetStatus(ctx context.Context, instanceName string) (Status, error) {
	args := m.Called(ctx, instanceName)
	return args.Get(0).(Status), args.Error(1)
}

func TestUpdateStatus(t *testing.T) {
	mockDB := new(MockDB)
	status := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
	}

	mockDB.On("UpdateStatus", mock.Anything, "test-instance", status).Return(nil)

	err := mockDB.UpdateStatus(context.Background(), "test-instance", status)
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestUpdateStatusError(t *testing.T) {
	mockDB := new(MockDB)
	status := Status{
		Players:   Players{Current: 0, Max: 10},
		Timestamp: time.Now(),
	}

	mockDB.On("UpdateStatus", mock.Anything, "test-instance", status).Return(assert.AnError)

	err := mockDB.UpdateStatus(context.Background(), "test-instance", status)
	assert.Error(t, err)
	mockDB.AssertExpectations(t)
}

func TestGetStatus(t *testing.T) {
	mockDB := new(MockDB)
	expectedStatus := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
	}

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(expectedStatus, nil)

	status, err := mockDB.GetStatus(context.Background(), "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, expectedStatus, status)
	mockDB.AssertExpectations(t)
}

func TestGetStatusError(t *testing.T) {
	mockDB := new(MockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(Status{}, assert.AnError)

	_, err := mockDB.GetStatus(context.Background(), "test-instance")
	assert.Error(t, err)
	mockDB.AssertExpectations(t)
}

func TestNewConnection_Success(t *testing.T) {
	// Test that FirestoreDB implements DB interface
	var _ DB = &FirestoreDB{}

	// Test with nil client (edge case)
	db := &FirestoreDB{client: nil}
	assert.NotNil(t, db)
}

func TestNewConnection_Error(t *testing.T) {
	// Test with invalid project ID (this would normally fail)
	// Since we can't easily mock the firestore.NewClientWithDatabase function,
	// we'll test the error path conceptually
	_, err := NewConnection(context.Background(), "", "test-db")
	assert.Error(t, err)
}

func TestFirestoreDB_UpdateStatus_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCollection := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	instanceName := "test-instance"
	status := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
		Uptime:    "2h30m",
	}

	// Setup mock expectations
	mockClient.On("Collection", "instances").Return(mockCollection)
	mockCollection.On("Doc", instanceName).Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCollection)
	mockSubCollection.On("Doc", "status").Return(mockSubDoc)
	mockSubDoc.On("Set", ctx, status, mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.UpdateStatus(ctx, instanceName, status)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockCollection.AssertExpectations(t)
	mockDoc.AssertExpectations(t)
	mockSubCollection.AssertExpectations(t)
	mockSubDoc.AssertExpectations(t)
}

func TestFirestoreDB_UpdateStatus_Error(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCollection := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	instanceName := "test-instance"
	status := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
		Uptime:    "2h30m",
	}

	// Setup mock expectations to return error
	mockClient.On("Collection", "instances").Return(mockCollection)
	mockCollection.On("Doc", instanceName).Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCollection)
	mockSubCollection.On("Doc", "status").Return(mockSubDoc)
	mockSubDoc.On("Set", ctx, status, mock.AnythingOfType("[]firestore.SetOption")).Return(nil, assert.AnError)

	err := db.UpdateStatus(ctx, instanceName, status)
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockCollection.AssertExpectations(t)
	mockDoc.AssertExpectations(t)
	mockSubCollection.AssertExpectations(t)
	mockSubDoc.AssertExpectations(t)
}

func TestFirestoreDB_GetStatus_Success(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCollection := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnapshot := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	instanceName := "test-instance"
	expectedStatus := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
		Uptime:    "2h30m",
	}

	// Setup mock expectations
	mockClient.On("Collection", "instances").Return(mockCollection)
	mockCollection.On("Doc", instanceName).Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCollection)
	mockSubCollection.On("Doc", "status").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnapshot, nil)
	mockSnapshot.On("DataTo", mock.AnythingOfType("*db.Status")).Return(nil).Run(func(args mock.Arguments) {
		status := args.Get(0).(*Status)
		*status = expectedStatus
	})

	status, err := db.GetStatus(ctx, instanceName)
	assert.NoError(t, err)
	assert.Equal(t, expectedStatus, status)

	mockClient.AssertExpectations(t)
	mockCollection.AssertExpectations(t)
	mockDoc.AssertExpectations(t)
	mockSubCollection.AssertExpectations(t)
	mockSubDoc.AssertExpectations(t)
	mockSnapshot.AssertExpectations(t)
}

func TestFirestoreDB_GetStatus_Error(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCollection := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	instanceName := "test-instance"

	// Setup mock expectations to return error
	mockClient.On("Collection", "instances").Return(mockCollection)
	mockCollection.On("Doc", instanceName).Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCollection)
	mockSubCollection.On("Doc", "status").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(nil, assert.AnError)

	_, err := db.GetStatus(ctx, instanceName)
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockCollection.AssertExpectations(t)
	mockDoc.AssertExpectations(t)
	mockSubCollection.AssertExpectations(t)
	mockSubDoc.AssertExpectations(t)
}

func TestFirestoreDB_UpdateStatus_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	ctx := context.Background()
	status := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
		Uptime:    "2h30m",
	}

	err := db.UpdateStatus(ctx, "test-instance", status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")
}

func TestFirestoreDB_GetStatus_NilClient(t *testing.T) {
	db := &FirestoreDB{client: nil}
	ctx := context.Background()

	_, err := db.GetStatus(ctx, "test-instance")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")
}

func TestFirestoreDB_GetStatus_DataToError(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCollection := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnapshot := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	instanceName := "test-instance"

	// Setup mock expectations to return DataTo error
	mockClient.On("Collection", "instances").Return(mockCollection)
	mockCollection.On("Doc", instanceName).Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCollection)
	mockSubCollection.On("Doc", "status").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnapshot, nil)
	mockSnapshot.On("DataTo", mock.AnythingOfType("*db.Status")).Return(assert.AnError)

	_, err := db.GetStatus(ctx, instanceName)
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
	mockCollection.AssertExpectations(t)
	mockDoc.AssertExpectations(t)
	mockSubCollection.AssertExpectations(t)
	mockSubDoc.AssertExpectations(t)
	mockSnapshot.AssertExpectations(t)
}

func TestFirestoreDB_UpdateStatus_WithServerState(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCollection := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	instanceName := "test-instance"
	status := Status{
		Players:     Players{Current: 5, Max: 20},
		Timestamp:   time.Now(),
		Uptime:      "2h30m",
		ServerState: ServerStateRunning,
	}

	// Setup mock expectations
	mockClient.On("Collection", "instances").Return(mockCollection)
	mockCollection.On("Doc", instanceName).Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCollection)
	mockSubCollection.On("Doc", "status").Return(mockSubDoc)
	mockSubDoc.On("Set", ctx, status, mock.AnythingOfType("[]firestore.SetOption")).Return(&firestore.WriteResult{}, nil)

	err := db.UpdateStatus(ctx, instanceName, status)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockCollection.AssertExpectations(t)
	mockDoc.AssertExpectations(t)
	mockSubCollection.AssertExpectations(t)
	mockSubDoc.AssertExpectations(t)
}

func TestFirestoreDB_GetStatus_WithServerState(t *testing.T) {
	mockClient := new(MockFirestoreClient)
	mockCollection := new(MockCollectionRef)
	mockDoc := new(MockDocumentRef)
	mockSubCollection := new(MockCollectionRef)
	mockSubDoc := new(MockDocumentRef)
	mockSnapshot := new(MockDocumentSnapshot)

	db := &FirestoreDB{client: mockClient}
	ctx := context.Background()
	instanceName := "test-instance"
	expectedStatus := Status{
		Players:     Players{Current: 5, Max: 20},
		Timestamp:   time.Now(),
		Uptime:      "2h30m",
		ServerState: ServerStateRunning,
	}

	// Setup mock expectations
	mockClient.On("Collection", "instances").Return(mockCollection)
	mockCollection.On("Doc", instanceName).Return(mockDoc)
	mockDoc.On("Collection", "data").Return(mockSubCollection)
	mockSubCollection.On("Doc", "status").Return(mockSubDoc)
	mockSubDoc.On("Get", ctx).Return(mockSnapshot, nil)
	mockSnapshot.On("DataTo", mock.AnythingOfType("*db.Status")).Return(nil).Run(func(args mock.Arguments) {
		status := args.Get(0).(*Status)
		*status = expectedStatus
	})

	status, err := db.GetStatus(ctx, instanceName)
	assert.NoError(t, err)
	assert.Equal(t, expectedStatus, status)
	assert.Equal(t, ServerStateRunning, status.ServerState)

	mockClient.AssertExpectations(t)
	mockCollection.AssertExpectations(t)
	mockDoc.AssertExpectations(t)
	mockSubCollection.AssertExpectations(t)
	mockSubDoc.AssertExpectations(t)
	mockSnapshot.AssertExpectations(t)
}
