package pulumi

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optimport"
	pulumiSdk "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// WorkspaceManagerInterface defines the operations needed for infrastructure provisioning.
// This interface allows for mocking in tests.
type WorkspaceManagerInterface interface {
	UpsertStack(ctx context.Context, name string, program func(*pulumiSdk.Context) error) (*auto.Stack, error)
	UpStack(ctx context.Context, stack *auto.Stack) (auto.UpResult, error)
	DestroyStack(ctx context.Context, name string) error
	CancelStack(ctx context.Context, name string) error
	RefreshStack(ctx context.Context, name string) error
	SetConfig(ctx context.Context, stack *auto.Stack, key, value string, secret bool) error
	ImportResources(ctx context.Context, stack *auto.Stack, resources []*optimport.ImportResource) error
	ProjectID() string
}
