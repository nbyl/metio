package services

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/testutil"
	"google.golang.org/api/googleapi"
)

type mockBucketHandle struct {
	attrsErr  error
	attrs     *storage.BucketAttrs
	createErr error
	attrsCalled bool
	createCalled bool
}

func (m *mockBucketHandle) Attrs(ctx context.Context) (*storage.BucketAttrs, error) {
	m.attrsCalled = true
	return m.attrs, m.attrsErr
}

func (m *mockBucketHandle) Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) error {
	m.createCalled = true
	return m.createErr
}

type mockStorageClient struct {
	buckets map[string]*mockBucketHandle
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{buckets: make(map[string]*mockBucketHandle)}
}

func (m *mockStorageClient) Bucket(name string) StorageBucketHandle {
	b, ok := m.buckets[name]
	if !ok {
		b = &mockBucketHandle{}
		m.buckets[name] = b
	}
	return b
}

func (m *mockStorageClient) bucket(name string) *mockBucketHandle {
	b, ok := m.buckets[name]
	if !ok {
		b = &mockBucketHandle{}
		m.buckets[name] = b
	}
	return b
}

func TestEnsureStateBucket_FirestoreHappyPath(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrs = &storage.BucketAttrs{
		Labels: map[string]string{
			"managed-by": "metio",
			"purpose":    "pulumi-state",
		},
	}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	settings := &db.PulumiSettings{
		StateBucket: "dev-metio-pulumi-state",
		Initialized: true,
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(settings, nil)

	svc := NewSetupService(cfg, mockDB, ms)
	name, err := svc.EnsureStateBucket(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "dev-metio-pulumi-state", name)
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_AutoCreate(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrsErr = &googleapi.Error{Code: 404}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, nil)
	mockDB.On("SetPulumiSettings", context.Background(), mock.AnythingOfType("*db.PulumiSettings")).Return(nil)

	svc := NewSetupService(cfg, mockDB, ms)
	name, err := svc.EnsureStateBucket(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "dev-metio-pulumi-state", name)
	assert.True(t, ms.bucket("dev-metio-pulumi-state").createCalled)
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_AdoptExistingLabeledBucket(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrs = &storage.BucketAttrs{
		Labels: map[string]string{
			"managed-by": "metio",
			"purpose":    "pulumi-state",
		},
	}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, nil)
	mockDB.On("SetPulumiSettings", context.Background(), mock.AnythingOfType("*db.PulumiSettings")).Return(nil)

	svc := NewSetupService(cfg, mockDB, ms)
	name, err := svc.EnsureStateBucket(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "dev-metio-pulumi-state", name)
	assert.False(t, ms.bucket("dev-metio-pulumi-state").createCalled)
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_BucketExistsWithoutLabels(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrs = &storage.BucketAttrs{}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, nil)

	svc := NewSetupService(cfg, mockDB, ms)
	_, err := svc.EnsureStateBucket(context.Background())

	assert.True(t, errors.Is(err, ErrBucketExistsWithoutLabels))
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_BucketExistsWithWrongLabels(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrs = &storage.BucketAttrs{
		Labels: map[string]string{
			"managed-by": "other",
		},
	}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, nil)

	svc := NewSetupService(cfg, mockDB, ms)
	_, err := svc.EnsureStateBucket(context.Background())

	assert.True(t, errors.Is(err, ErrBucketExistsWithoutLabels))
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_InsufficientPermissions(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrsErr = &googleapi.Error{Code: 403, Message: "permission denied"}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, nil)

	svc := NewSetupService(cfg, mockDB, ms)
	_, err := svc.EnsureStateBucket(context.Background())

	assert.True(t, errors.Is(err, ErrInsufficientPermissions))
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_BillingNotEnabled(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrsErr = &googleapi.Error{Code: 403, Message: "billing not enabled"}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, nil)

	svc := NewSetupService(cfg, mockDB, ms)
	_, err := svc.EnsureStateBucket(context.Background())

	assert.True(t, errors.Is(err, ErrBillingNotEnabled))
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_QuotaExceeded(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrsErr = &googleapi.Error{Code: 429, Message: "quota exceeded"}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, nil)

	svc := NewSetupService(cfg, mockDB, ms)
	_, err := svc.EnsureStateBucket(context.Background())

	assert.True(t, errors.Is(err, ErrQuotaExceeded))
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_CreateFailsWith403(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrsErr = &googleapi.Error{Code: 404}
	ms.bucket("dev-metio-pulumi-state").createErr = &googleapi.Error{Code: 403, Message: "permission denied"}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, nil)

	svc := NewSetupService(cfg, mockDB, ms)
	_, err := svc.EnsureStateBucket(context.Background())

	assert.True(t, errors.Is(err, ErrInsufficientPermissions))
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_FirestoreReadError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, errors.New("firestore unavailable"))

	svc := NewSetupService(cfg, mockDB, ms)
	_, err := svc.EnsureStateBucket(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read Pulumi settings")
	mockDB.AssertExpectations(t)
}

func TestEnsureStateBucket_FirestoreWriteError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	ms := newMockStorageClient()
	ms.bucket("dev-metio-pulumi-state").attrsErr = &googleapi.Error{Code: 404}

	cfg := config.Config{
		ProjectID:   "test-project",
		Region:      "us-central1",
		Environment: "dev",
	}

	mockDB.On("GetPulumiSettings", context.Background()).Return(nil, nil)
	mockDB.On("SetPulumiSettings", context.Background(), mock.AnythingOfType("*db.PulumiSettings")).Return(errors.New("firestore write failed"))

	svc := NewSetupService(cfg, mockDB, ms)
	_, err := svc.EnsureStateBucket(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to persist Pulumi settings")
	mockDB.AssertExpectations(t)
}
