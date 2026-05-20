package services

import (
	"context"

	"cloud.google.com/go/storage"
)

type StorageClient interface {
	Bucket(name string) StorageBucketHandle
}

type StorageBucketHandle interface {
	Attrs(ctx context.Context) (*storage.BucketAttrs, error)
	Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) error
}

type StorageAdapter struct {
	client *storage.Client
}

func NewStorageAdapter(client *storage.Client) *StorageAdapter {
	return &StorageAdapter{client: client}
}

func (a *StorageAdapter) Bucket(name string) StorageBucketHandle {
	return &BucketAdapter{bucket: a.client.Bucket(name)}
}

type BucketAdapter struct {
	bucket *storage.BucketHandle
}

func (a *BucketAdapter) Attrs(ctx context.Context) (*storage.BucketAttrs, error) {
	return a.bucket.Attrs(ctx)
}

func (a *BucketAdapter) Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) error {
	return a.bucket.Create(ctx, projectID, attrs)
}
