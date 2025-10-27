package db

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/mock"
)

// MockFirestoreClient mocks the FirestoreClient interface
type MockFirestoreClient struct {
	mock.Mock
}

func (m *MockFirestoreClient) Collection(name string) CollectionRef {
	args := m.Called(name)
	return args.Get(0).(CollectionRef)
}

// MockCollectionRef mocks the CollectionRef interface
type MockCollectionRef struct {
	mock.Mock
}

func (m *MockCollectionRef) Doc(id string) DocumentRef {
	args := m.Called(id)
	return args.Get(0).(DocumentRef)
}

// MockDocumentRef mocks the DocumentRef interface
type MockDocumentRef struct {
	mock.Mock
}

func (m *MockDocumentRef) Collection(id string) CollectionRef {
	args := m.Called(id)
	return args.Get(0).(CollectionRef)
}

func (m *MockDocumentRef) Set(ctx context.Context, data interface{}, opts ...firestore.SetOption) (*firestore.WriteResult, error) {
	args := m.Called(ctx, data, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*firestore.WriteResult), args.Error(1)
}

func (m *MockDocumentRef) Get(ctx context.Context) (DocumentSnapshot, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(DocumentSnapshot), args.Error(1)
}

// MockDocumentSnapshot mocks the DocumentSnapshot interface
type MockDocumentSnapshot struct {
	mock.Mock
}

func (m *MockDocumentSnapshot) DataTo(dst interface{}) error {
	args := m.Called(dst)
	return args.Error(0)
}
