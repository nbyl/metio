package pulumi

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optimport"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type WorkspaceManager struct {
	projectID   string
	stateBucket string
}

func NewWorkspaceManager(ctx context.Context, projectID, stateBucket string) (*WorkspaceManager, error) {
	if projectID == "" {
		return nil, fmt.Errorf("projectID is required")
	}
	if stateBucket == "" {
		return nil, fmt.Errorf("stateBucket is required")
	}

	os.Setenv("PULUMI_BACKEND_URL", fmt.Sprintf("gs://%s", stateBucket))
	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "")
	os.Setenv("PULUMI_HOME", "/tmp/.pulumi")

	return &WorkspaceManager{
		projectID:   projectID,
		stateBucket: stateBucket,
	}, nil
}

type StackManager struct {
	stack *auto.Stack
	wm    *WorkspaceManager
}

func (wm *WorkspaceManager) UpsertStack(ctx context.Context, name string, program func(*pulumi.Context) error) (*auto.Stack, error) {
	if name == "" {
		return nil, fmt.Errorf("stack name is required")
	}
	if program == nil {
		return nil, fmt.Errorf("program is required")
	}

	stack, err := auto.UpsertStackInlineSource(ctx, name, "metio", program)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert stack: %w", err)
	}

	return &stack, nil
}

func (wm *WorkspaceManager) SelectStack(ctx context.Context, name string) (*auto.Stack, error) {
	if name == "" {
		return nil, fmt.Errorf("stack name is required")
	}

	stack, err := auto.SelectStackInlineSource(ctx, name, "metio", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to select stack: %w", err)
	}

	return &stack, nil
}

func (wm *WorkspaceManager) CancelStack(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("stack name is required")
	}

	stack, err := auto.SelectStackInlineSource(ctx, name, "metio", func(_ *pulumi.Context) error {
		return nil
	})
	if err != nil {
		if IsSelectStack404Error(err) {
			return nil
		}
		return fmt.Errorf("failed to select stack for cancel: %w", err)
	}

	err = stack.Cancel(ctx)
	if err != nil {
		log.Printf("[%s] Cancel returned (non-fatal): %v", name, err)
	}

	return nil
}

func (wm *WorkspaceManager) DestroyStack(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("stack name is required")
	}

	stack, err := auto.SelectStackInlineSource(ctx, name, "metio", nil)
	if err != nil {
		return fmt.Errorf("failed to select stack for destroy: %w", err)
	}

	_, err = stack.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed to destroy stack: %w", err)
	}

	err = stack.Workspace().RemoveStack(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to remove stack: %w", err)
	}

	return nil
}

func (wm *WorkspaceManager) RefreshStack(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("stack name is required")
	}

	dummy := func(_ *pulumi.Context) error { return nil }
	stack, err := auto.SelectStackInlineSource(ctx, name, "metio", dummy)
	if err != nil {
		if auto.IsSelectStack404Error(err) {
			return nil
		}
		return fmt.Errorf("failed to select stack: %w", err)
	}

	_, err = stack.Refresh(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh stack: %w", err)
	}

	return nil
}

func (wm *WorkspaceManager) UpStack(ctx context.Context, stack *auto.Stack) (auto.UpResult, error) {
	if stack == nil {
		return auto.UpResult{}, fmt.Errorf("stack is required")
	}

	stdoutStreamer := optup.ProgressStreams(os.Stdout)

	result, err := stack.Up(ctx, stdoutStreamer)
	if err != nil {
		return auto.UpResult{}, fmt.Errorf("failed to up stack: %w", err)
	}

	return result, nil
}

func (wm *WorkspaceManager) InstallPlugin(ctx context.Context, stack *auto.Stack, name, version string) error {
	if stack == nil {
		return fmt.Errorf("stack is required")
	}
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if version == "" {
		version = "v0.0.0"
	}

	err := stack.Workspace().InstallPlugin(ctx, name, version)
	if err != nil {
		return fmt.Errorf("failed to install plugin %s@%s: %w", name, version, err)
	}

	return nil
}

func (wm *WorkspaceManager) SetConfig(ctx context.Context, stack *auto.Stack, key, value string, secret bool) error {
	if stack == nil {
		return fmt.Errorf("stack is required")
	}
	if key == "" {
		return fmt.Errorf("config key is required")
	}

	configValue := auto.ConfigValue{
		Value:  value,
		Secret: secret,
	}

	err := stack.SetConfig(ctx, key, configValue)
	if err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}

	return nil
}

func (wm *WorkspaceManager) ImportResources(ctx context.Context, stack *auto.Stack, resources []*optimport.ImportResource) error {
	if stack == nil {
		return fmt.Errorf("stack is required")
	}
	if len(resources) == 0 {
		return fmt.Errorf("at least one resource is required")
	}

	_, err := stack.ImportResources(ctx,
		optimport.Resources(resources),
		optimport.GenerateCode(false),
		optimport.ProgressStreams(os.Stdout),
	)
	if err != nil {
		return fmt.Errorf("failed to import resources: %w", err)
	}

	return nil
}

func (wm *WorkspaceManager) GetConfig(ctx context.Context, stack *auto.Stack, key string) (string, error) {
	if stack == nil {
		return "", fmt.Errorf("stack is required")
	}
	if key == "" {
		return "", fmt.Errorf("config key is required")
	}

	configValue, err := stack.GetConfig(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to get config: %w", err)
	}

	return configValue.Value, nil
}

func (wm *WorkspaceManager) ProjectID() string {
	return wm.projectID
}

func (wm *WorkspaceManager) StateBucket() string {
	return wm.stateBucket
}

func IsConcurrentUpdateError(err error) bool {
	return auto.IsConcurrentUpdateError(err)
}

func IsCreateStack409Error(err error) bool {
	return auto.IsCreateStack409Error(err)
}

func IsSelectStack404Error(err error) bool {
	return auto.IsSelectStack404Error(err)
}
