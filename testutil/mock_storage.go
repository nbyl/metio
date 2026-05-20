package testutil

import (
	"context"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
)

type StorageBucketHandle interface {
	Attrs(ctx context.Context) (*storage.BucketAttrs, error)
	Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) error
}

type MockStorageClient struct {
	mock.Mock
}

func (m *MockStorageClient) Bucket(name string) StorageBucketHandle {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(StorageBucketHandle)
}

type MockBucketHandle struct {
	mock.Mock
}

func (m *MockBucketHandle) Attrs(ctx context.Context) (*storage.BucketAttrs, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.BucketAttrs), args.Error(1)
}

func (m *MockBucketHandle) Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) error {
	args := m.Called(ctx, projectID, attrs)
	return args.Error(0)
}
