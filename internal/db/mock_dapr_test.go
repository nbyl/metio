package db

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockDaprClient struct {
	mock.Mock
}

func (m *MockDaprClient) Save(ctx context.Context, storeName, key string, data []byte) error {
	args := m.Called(ctx, storeName, key, data)
	return args.Error(0)
}

func (m *MockDaprClient) Get(ctx context.Context, storeName, key string) (*daprStateItem, error) {
	args := m.Called(ctx, storeName, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*daprStateItem), args.Error(1)
}

func (m *MockDaprClient) GetBulk(ctx context.Context, storeName string, keys []string) ([]*daprBulkStateItem, error) {
	args := m.Called(ctx, storeName, keys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*daprBulkStateItem), args.Error(1)
}

func (m *MockDaprClient) Delete(ctx context.Context, storeName, key string) error {
	args := m.Called(ctx, storeName, key)
	return args.Error(0)
}
