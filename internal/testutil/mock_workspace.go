package testutil

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	pulumiSdk "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/mock"
)

// MockWorkspaceManager is a testify mock implementation of pulumi.WorkspaceManagerInterface.
type MockWorkspaceManager struct {
	mock.Mock
}

func (m *MockWorkspaceManager) UpsertStack(ctx context.Context, name string, program func(*pulumiSdk.Context) error) (*auto.Stack, error) {
	args := m.Called(ctx, name, program)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auto.Stack), args.Error(1)
}

func (m *MockWorkspaceManager) UpStack(ctx context.Context, stack *auto.Stack) (auto.UpResult, error) {
	args := m.Called(ctx, stack)
	return args.Get(0).(auto.UpResult), args.Error(1)
}

func (m *MockWorkspaceManager) DestroyStack(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockWorkspaceManager) CancelStack(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockWorkspaceManager) RefreshStack(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockWorkspaceManager) SetConfig(ctx context.Context, stack *auto.Stack, key, value string, secret bool) error {
	args := m.Called(ctx, stack, key, value, secret)
	return args.Error(0)
}

func (m *MockWorkspaceManager) ProjectID() string {
	args := m.Called()
	return args.String(0)
}
