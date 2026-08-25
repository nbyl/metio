package services

import (
	"context"
	"errors"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type StorageClient interface {
	Bucket(name string) StorageBucketHandle
}

type StorageBucketHandle interface {
	Attrs(ctx context.Context) (*storage.BucketAttrs, error)
	Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) error
	DeletePrefix(ctx context.Context, prefix string) (int64, error)
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

func (a *BucketAdapter) DeletePrefix(ctx context.Context, prefix string) (int64, error) {
	it := a.bucket.Objects(ctx, &storage.Query{Prefix: prefix})
	var deleted int64
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return deleted, nil
		}
		if err != nil {
			return deleted, err
		}
		if err := a.bucket.Object(attrs.Name).Delete(ctx); err != nil {
			if errors.Is(err, storage.ErrObjectNotExist) {
				continue
			}
			return deleted, err
		}
		deleted++
	}
}
