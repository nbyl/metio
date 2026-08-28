package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/pulumi"
	"github.com/nbyl/metio/internal/pulumi/programs"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

const (
	stepUpsertStack          = "upsert_stack"
	stepDeployInfrastructure = "deploy_infrastructure"
	stepRefreshStack         = "refresh_stack"
	stepUpdateInfrastructure = "update_infrastructure"
	stepDestroyStack         = "destroy_stack"
	stepStopInstance         = "stop_instance"
	stepStartInstance        = "start_instance"
	stepSaveWorld            = "save_world"
	stepRestoreWorld         = "restore_world"
	stepHealthCheck          = "health_check"
)

const (
	updateTypeInPlace  = 0
	updateTypeResize   = 1
	updateTypeRecreate = 2
)

const errMsgOperationInProgress = "operation already in progress for server %s"
const errMsgNoOperationInProgress = "no operation in progress for server %s"
const errMsgRetryExhausted = "operation failed after %d attempts: %w"

var errOperationInProgress = errors.New("operation already in progress")
var errNoOperationInProgress = errors.New("no operation in progress")

type ProvisioningService struct {
	workspaceManager    pulumi.WorkspaceManagerInterface
	db                  db.DB
	backupCoord         *BackupCoordinator
	controllerVersion   string
	executor            OperationExecutor
	retryAttempts       int
	retryDelay          time.Duration
	backupRetentionDays int
	restoreAckTimeout   time.Duration
	saveAckTimeout      time.Duration
}

func (s *ProvisioningService) OperationTimeout() time.Duration {
	return s.executor.OperationTimeout()
}

func NewProvisioningService(workspaceManager pulumi.WorkspaceManagerInterface, dbConn db.DB, controllerVersion string, executor OperationExecutor, backupRetentionDays int) *ProvisioningService {
	return &ProvisioningService{
		workspaceManager:    workspaceManager,
		db:                  dbConn,
		backupCoord:         NewBackupCoordinator(dbConn),
		controllerVersion:   controllerVersion,
		executor:            executor,
		retryAttempts:       3,
		retryDelay:          5 * time.Second,
		backupRetentionDays: backupRetentionDays,
		restoreAckTimeout:   30 * time.Minute,
		saveAckTimeout:      config.DefaultSaveAckTimeout,
	}
}

func (s *ProvisioningService) SetSaveAckTimeout(d time.Duration) {
	if d > 0 {
		s.saveAckTimeout = d
	}
}

func (s *ProvisioningService) CreateServer(ctx context.Context, serverID string, config *programs.ServerConfig) error {
	return s.executor.StartOperation(ctx, serverID, db.ProvisioningOperationCreate, nil, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		return s.runCreate(opCtx, status, serverID, config)
	})
}

// CreateServerFromBackup creates a new server and restores a snapshot before
// Minecraft starts. The restore information is carried on the ServerConfig
// (RestoreSnapshotID / RestoreSourcePrefix) and baked into the cloud-config
// so it runs during the VM's first boot. It is also seeded into the operation
// status outputs so that deferred execution (e.g. via Cloud Tasks) can rebuild
// the full program config when the operation actually runs.
func (s *ProvisioningService) CreateServerFromBackup(ctx context.Context, serverID string, config *programs.ServerConfig) error {
	initialOutputs := map[string]string{}
	if config.RestoreSnapshotID != "" {
		initialOutputs["restoreSnapshotId"] = config.RestoreSnapshotID
	}
	if config.RestoreSourcePrefix != "" {
		initialOutputs["restoreSourcePrefix"] = config.RestoreSourcePrefix
	}
	return s.executor.StartOperation(ctx, serverID, db.ProvisioningOperationCreate, initialOutputs, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		return s.runCreate(opCtx, status, serverID, config)
	})
}

func (s *ProvisioningService) UpdateServer(ctx context.Context, serverID string, config *programs.ServerConfig, updateType int) error {
	return s.executor.StartOperation(ctx, serverID, db.ProvisioningOperationUpdate, nil, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		return s.runUpdate(opCtx, status, serverID, config, updateType)
	})
}

func (s *ProvisioningService) DestroyServer(ctx context.Context, serverID string) error {
	return s.executor.StartOperation(ctx, serverID, db.ProvisioningOperationDestroy, nil, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		return s.runDestroy(opCtx, status, serverID)
	})
}

// RestoreServer restores the given backup onto an existing server. The backup must
// already be validated by the caller (ownership and COMPLETED status).
func (s *ProvisioningService) RestoreServer(ctx context.Context, serverID string, backup *db.Backup, versionWarning string) error {
	initialOutputs := map[string]string{
		"backupId":   backup.ID,
		"snapshotId": backup.SnapshotID,
	}
	if versionWarning != "" {
		initialOutputs["versionMismatchWarning"] = versionWarning
	}
	return s.executor.StartOperation(ctx, serverID, db.ProvisioningOperationRestore, initialOutputs, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		return s.runRestore(opCtx, status, serverID)
	})
}

func (s *ProvisioningService) runRestore(opCtx context.Context, status *db.ProvisioningStatus, serverID string) error {
	status.Steps = []db.ProvisioningStep{
		{Name: stepSaveWorld, Status: db.ProvisioningStatePending, Message: "Saving world data...", Timestamp: time.Now()},
		{Name: stepRestoreWorld, Status: db.ProvisioningStatePending, Message: "Restoring backup...", Timestamp: time.Now()},
		{Name: stepHealthCheck, Status: db.ProvisioningStatePending, Message: "Waiting for server to become healthy...", Timestamp: time.Now()},
	}
	s.updateStatus(opCtx, serverID, status)

	config, err := s.db.GetServerConfig(opCtx, serverID)
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepSaveWorld,
			fmt.Errorf("failed to load server config: %w", err))
	}

	snapshotID := status.Outputs["snapshotId"]
	if snapshotID == "" {
		return s.handleError(status, opCtx, serverID, stepSaveWorld,
			fmt.Errorf("missing snapshot id in operation outputs"))
	}

	s.updateStep(opCtx, status, serverID, stepSaveWorld, "Saving world data...")
	if err := s.backupCoord.TriggerWorldSave(opCtx, config.Name); err != nil {
		return s.handleError(status, opCtx, serverID, stepSaveWorld,
			fmt.Errorf("failed to trigger world save: %w", err))
	}
	result, err := s.backupCoord.WaitForCommandAck(opCtx, config.Name, s.saveAckTimeout)
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepSaveWorld,
			fmt.Errorf("world save failed: result=%s, %w", result, err))
	}
	s.completeStep(opCtx, status, serverID, stepSaveWorld)

	s.updateStep(opCtx, status, serverID, stepRestoreWorld, fmt.Sprintf("Restoring snapshot %s on machine agent...", snapshotID))
	if err := s.backupCoord.TriggerRestore(opCtx, config.Name, snapshotID); err != nil {
		return s.handleError(status, opCtx, serverID, stepRestoreWorld,
			fmt.Errorf("failed to trigger restore: %w", err))
	}
	result, err = s.backupCoord.WaitForCommandAck(opCtx, config.Name, s.restoreAckTimeout)
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepRestoreWorld,
			fmt.Errorf("restore failed: result=%s, %w", result, err))
	}
	s.completeStep(opCtx, status, serverID, stepRestoreWorld)

	s.updateStep(opCtx, status, serverID, stepHealthCheck, "Waiting for server to become healthy...")
	if err := WaitForServerHealthy(opCtx, s.db, config.Name, 180*time.Second); err != nil {
		log.Printf("Warning: server %s did not become healthy after restore: %v", serverID, err)
	}
	s.completeStep(opCtx, status, serverID, stepHealthCheck)

	status.State = db.ProvisioningStateCompleted
	s.updateStatus(opCtx, serverID, status)

	return nil
}

func (s *ProvisioningService) runCreate(opCtx context.Context, status *db.ProvisioningStatus, serverID string, config *programs.ServerConfig) error {
	if err := s.workspaceManager.CancelStack(opCtx, serverID); err != nil {
		log.Printf("[%s] Failed to cancel stale stack (non-fatal): %v", serverID, err)
	}

	status.Steps = []db.ProvisioningStep{
		{Name: stepUpsertStack, Status: db.ProvisioningStatePending, Message: "Preparing Pulumi stack...", Timestamp: time.Now()},
		{Name: stepDeployInfrastructure, Status: db.ProvisioningStatePending, Message: "Deploying infrastructure with Pulumi...", Timestamp: time.Now()},
	}
	s.updateStatus(opCtx, serverID, status)

	// Adopt a pre-existing address on create only. Updates must never re-import
	// an address that is already managed by the stack.
	createConfig := *config
	createConfig.ImportExistingAddress = config.ExistingAddress != ""

	stack, err := s.workspaceManager.UpsertStack(opCtx, serverID, programs.ServerProgram(&createConfig))
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepUpsertStack, err)
	}

	if err := s.workspaceManager.SetConfig(opCtx, stack, "gcp:project", s.workspaceManager.ProjectID(), false); err != nil {
		return s.handleError(status, opCtx, serverID, stepUpsertStack, err)
	}

	s.completeStep(opCtx, status, serverID, stepUpsertStack)

	s.updateStep(opCtx, status, serverID, stepDeployInfrastructure, "Deploying infrastructure with Pulumi...")
	result, err := s.executeUpWithRetry(opCtx, func() (auto.UpResult, error) {
		return s.workspaceManager.UpStack(opCtx, stack)
	})
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepDeployInfrastructure, err)
	}

	s.completeStep(opCtx, status, serverID, stepDeployInfrastructure)

	if err := s.stampServerConfig(opCtx, serverID); err != nil {
		return s.handleError(status, opCtx, serverID, stepDeployInfrastructure, err)
	}

	outputs := make(map[string]string)
	for key, value := range result.Outputs {
		if str, ok := value.Value.(string); ok {
			outputs[key] = str
		}
	}

	status.Outputs = outputs
	status.State = db.ProvisioningStateCompleted
	s.updateStatus(opCtx, serverID, status)

	return nil
}

func (s *ProvisioningService) runUpdate(opCtx context.Context, status *db.ProvisioningStatus, serverID string, config *programs.ServerConfig, updateType int) error {
	switch updateType {
	case updateTypeResize:
		return s.runResizeUpdate(opCtx, status, serverID, config)
	case updateTypeRecreate:
		return s.runRecreateUpdate(opCtx, status, serverID, config)
	default:
		return s.runInPlaceUpdate(opCtx, status, serverID, config)
	}
}

func (s *ProvisioningService) runInPlaceUpdate(opCtx context.Context, status *db.ProvisioningStatus, serverID string, config *programs.ServerConfig) error {
	return s.runPulumiUpdate(opCtx, status, serverID, config)
}

func (s *ProvisioningService) runResizeUpdate(opCtx context.Context, status *db.ProvisioningStatus, serverID string, config *programs.ServerConfig) error {
	status.Steps = []db.ProvisioningStep{
		{Name: stepStopInstance, Status: db.ProvisioningStatePending, Message: "Stopping VM instance...", Timestamp: time.Now()},
		{Name: stepUpsertStack, Status: db.ProvisioningStatePending, Message: "Updating infrastructure...", Timestamp: time.Now()},
		{Name: stepStartInstance, Status: db.ProvisioningStatePending, Message: "Starting VM instance...", Timestamp: time.Now()},
	}
	s.updateStatus(opCtx, serverID, status)

	s.updateStep(opCtx, status, serverID, stepStopInstance, "Stopping VM instance...")
	if err := StopInstance(opCtx, config.GCPProject, config.Zone, config.Name); err != nil {
		return s.handleError(status, opCtx, serverID, stepStopInstance, err)
	}
	s.completeStep(opCtx, status, serverID, stepStopInstance)

	if err := s.runPulumiUpdate(opCtx, status, serverID, config); err != nil {
		if startErr := StartInstance(opCtx, config.GCPProject, config.Zone, config.Name); startErr != nil {
			log.Printf("Failed to restart instance %s after update failure: %v", serverID, startErr)
		}
		return err
	}

	s.updateStep(opCtx, status, serverID, stepStartInstance, "Starting VM instance...")
	if err := StartInstance(opCtx, config.GCPProject, config.Zone, config.Name); err != nil {
		return s.handleError(status, opCtx, serverID, stepStartInstance, err)
	}
	s.completeStep(opCtx, status, serverID, stepStartInstance)

	s.updateStep(opCtx, status, serverID, stepHealthCheck, "Waiting for server to become healthy...")
	if err := WaitForServerHealthy(opCtx, s.db, config.Name, 120*time.Second); err != nil {
		log.Printf("Warning: server %s did not become healthy after resize: %v", serverID, err)
	}
	s.completeStep(opCtx, status, serverID, stepHealthCheck)

	return nil
}

func (s *ProvisioningService) runRecreateUpdate(opCtx context.Context, status *db.ProvisioningStatus, serverID string, config *programs.ServerConfig) error {
	status.Steps = []db.ProvisioningStep{
		{Name: stepSaveWorld, Status: db.ProvisioningStatePending, Message: "Saving world data...", Timestamp: time.Now()},
		{Name: stepUpsertStack, Status: db.ProvisioningStatePending, Message: "Updating infrastructure...", Timestamp: time.Now()},
	}
	s.updateStatus(opCtx, serverID, status)

	s.updateStep(opCtx, status, serverID, stepSaveWorld, "Saving world data...")
	if err := s.backupCoord.TriggerWorldSave(opCtx, config.Name); err != nil {
		return s.handleError(status, opCtx, serverID, stepSaveWorld,
			fmt.Errorf("failed to trigger world save: %w", err))
	}
	result, err := s.backupCoord.WaitForCommandAck(opCtx, config.Name, s.saveAckTimeout)
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepSaveWorld,
			fmt.Errorf("world save failed: result=%s, %w", result, err))
	}
	s.completeStep(opCtx, status, serverID, stepSaveWorld)

	if err := s.runPulumiUpdate(opCtx, status, serverID, config); err != nil {
		return err
	}

	s.updateStep(opCtx, status, serverID, stepHealthCheck, "Waiting for server to become healthy...")
	if err := WaitForServerHealthy(opCtx, s.db, config.Name, 180*time.Second); err != nil {
		log.Printf("Warning: server %s did not become healthy after recreate: %v", serverID, err)
	}
	s.completeStep(opCtx, status, serverID, stepHealthCheck)

	return nil
}

func (s *ProvisioningService) runPulumiUpdate(opCtx context.Context, status *db.ProvisioningStatus, serverID string, config *programs.ServerConfig) error {
	if err := s.workspaceManager.CancelStack(opCtx, serverID); err != nil {
		log.Printf("[%s] Failed to cancel stale stack (non-fatal): %v", serverID, err)
	}

	stack, err := s.workspaceManager.UpsertStack(opCtx, serverID, programs.ServerProgram(config))
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepUpsertStack, err)
	}

	if err := s.workspaceManager.SetConfig(opCtx, stack, "gcp:project", s.workspaceManager.ProjectID(), false); err != nil {
		return s.handleError(status, opCtx, serverID, stepUpsertStack, err)
	}

	s.completeStep(opCtx, status, serverID, stepUpsertStack)

	s.updateStep(opCtx, status, serverID, stepRefreshStack, "Refreshing stack state...")
	if err := s.workspaceManager.RefreshStack(opCtx, serverID); err != nil {
		log.Printf("[%s] Failed to refresh stack (non-fatal): %v", serverID, err)
	}
	s.completeStep(opCtx, status, serverID, stepRefreshStack)

	s.updateStep(opCtx, status, serverID, stepUpdateInfrastructure, "Updating infrastructure...")
	result, err := s.executeUpWithRetry(opCtx, func() (auto.UpResult, error) {
		return s.workspaceManager.UpStack(opCtx, stack)
	})
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepUpdateInfrastructure, err)
	}

	s.completeStep(opCtx, status, serverID, stepUpdateInfrastructure)

	if err := s.stampServerConfig(opCtx, serverID); err != nil {
		return s.handleError(status, opCtx, serverID, stepUpdateInfrastructure, err)
	}

	outputs := make(map[string]string)
	for key, value := range result.Outputs {
		if str, ok := value.Value.(string); ok {
			outputs[key] = str
		}
	}

	status.Outputs = outputs
	status.State = db.ProvisioningStateCompleted
	s.updateStatus(opCtx, serverID, status)

	return nil
}

func (s *ProvisioningService) runDestroy(opCtx context.Context, status *db.ProvisioningStatus, serverID string) error {
	if err := s.workspaceManager.CancelStack(opCtx, serverID); err != nil {
		log.Printf("[%s] Failed to cancel stale stack (non-fatal): %v", serverID, err)
	}

	steps := []db.ProvisioningStep{
		{
			Name: stepDestroyStack, Status: db.ProvisioningStatePending, Message: "Destroying Pulumi stack...", Timestamp: time.Now(),
		},
	}
	status.Steps = steps
	s.updateStatus(opCtx, serverID, status)

	s.updateStep(opCtx, status, serverID, stepDestroyStack, "Destroying Pulumi stack...")
	err := s.executeWithRetry(opCtx, func() error {
		return s.workspaceManager.DestroyStack(opCtx, serverID)
	})
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepDestroyStack, err)
	}

	s.completeStep(opCtx, status, serverID, stepDestroyStack)

	status.State = db.ProvisioningStateCompleted
	s.updateStatus(opCtx, serverID, status)

	config, err := s.db.GetServerConfig(opCtx, serverID)
	if err != nil {
		log.Printf("Failed to load server config for %s before deletion; backups will be stamped without source config: %v", serverID, err)
		config = nil
	}

	if err := s.db.DeleteServerConfig(opCtx, serverID); err != nil {
		log.Printf("Failed to delete server config for %s: %v", serverID, err)
	} else {
		var sourceConfig *db.BackupSourceConfig
		if config != nil {
			sourceConfig = &db.BackupSourceConfig{
				Region:           config.Region,
				Zone:             config.Zone,
				MachineType:      config.MachineType,
				DiskSizeGB:       config.DiskSizeGB,
				MinecraftVersion: config.MinecraftVersion,
			}
		}
		retentionUntil := time.Now().AddDate(0, 0, s.backupRetentionDays)
		if err := s.db.MarkServerBackupsDeleted(opCtx, serverID, sourceConfig, time.Now(), retentionUntil); err != nil {
			log.Printf("Failed to mark backups deleted for %s: %v", serverID, err)
		}
	}

	return nil
}

// ExecuteOperation runs a provisioning operation inline (bypassing the executor).
// Used by the Cloud Tasks task handler to process tasks in the request context.
func (s *ProvisioningService) ExecuteOperation(ctx context.Context, serverID string, config *programs.ServerConfig, updateType int) error {
	status, err := s.db.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		return fmt.Errorf("failed to get provisioning status: %w", err)
	}
	if status == nil {
		return fmt.Errorf("no provisioning status found for server %s", serverID)
	}

	log.Printf("[%s] Executing operation %s (updateType=%d)", serverID, status.Operation.String(), updateType)

	switch status.Operation {
	case db.ProvisioningOperationCreate:
		applyRestoreToConfig(config, status.Outputs)
		return s.runCreate(ctx, status, serverID, config)
	case db.ProvisioningOperationUpdate:
		return s.runUpdate(ctx, status, serverID, config, updateType)
	case db.ProvisioningOperationDestroy:
		return s.runDestroy(ctx, status, serverID)
	case db.ProvisioningOperationRestore:
		return s.runRestore(ctx, status, serverID)
	default:
		return fmt.Errorf("unknown operation type: %v", status.Operation)
	}
}

// applyRestoreToConfig carries restore fields persisted in the operation
// outputs onto a program config that was rebuilt from persisted server state
// (e.g. by the Cloud Tasks task handler). It only fills empty fields so a
// config that already carries the restore information is left untouched.
func applyRestoreToConfig(config *programs.ServerConfig, outputs map[string]string) {
	if config == nil || len(outputs) == 0 {
		return
	}
	if config.RestoreSnapshotID == "" {
		if v := outputs["restoreSnapshotId"]; v != "" {
			config.RestoreSnapshotID = v
		}
	}
	if config.RestoreSourcePrefix == "" {
		if v := outputs["restoreSourcePrefix"]; v != "" {
			config.RestoreSourcePrefix = v
		}
	}
}

func (s *ProvisioningService) RevertServerConfig(ctx context.Context, serverID string) error {
	snapshot, err := s.db.GetConfigSnapshot(ctx, serverID)
	if err != nil {
		return fmt.Errorf("failed to get config snapshot: %w", err)
	}
	if err := s.db.UpdateServerConfig(ctx, serverID, snapshot); err != nil {
		return fmt.Errorf("failed to revert server config: %w", err)
	}
	if err := s.db.DeleteConfigSnapshot(ctx, serverID); err != nil {
		log.Printf("Warning: failed to delete config snapshot for %s: %v", serverID, err)
	}
	return nil
}

func (s *ProvisioningService) CancelOperation(ctx context.Context, serverID string) error {
	return s.executor.CancelOperation(ctx, serverID)
}

func (s *ProvisioningService) GetProvisioningStatus(ctx context.Context, serverID string) (*db.ProvisioningStatus, error) {
	return s.db.GetProvisioningStatus(ctx, serverID)
}

func (s *ProvisioningService) executeWithRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < s.retryAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		if !isRetryableError(err) {
			return err
		}

		lastErr = err
		log.Printf("Retryable error (attempt %d/%d): %v", attempt+1, s.retryAttempts, err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.retryDelay):
		}
	}

	return fmt.Errorf(errMsgRetryExhausted, s.retryAttempts, lastErr)
}

func (s *ProvisioningService) executeUpWithRetry(ctx context.Context, fn func() (auto.UpResult, error)) (auto.UpResult, error) {
	var lastErr error
	for attempt := 0; attempt < s.retryAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return auto.UpResult{}, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		if !isRetryableError(err) {
			return auto.UpResult{}, err
		}

		lastErr = err
		log.Printf("Retryable error (attempt %d/%d): %v", attempt+1, s.retryAttempts, err)

		select {
		case <-ctx.Done():
			return auto.UpResult{}, ctx.Err()
		case <-time.After(s.retryDelay):
		}
	}

	return auto.UpResult{}, fmt.Errorf(errMsgRetryExhausted, s.retryAttempts, lastErr)
}

func (s *ProvisioningService) stampServerConfig(ctx context.Context, serverID string) error {
	config, err := s.db.GetServerConfig(ctx, serverID)
	if err != nil {
		return fmt.Errorf("failed to get server config: %w", err)
	}
	config.InfraVersion = programs.CurrentInfraVersion
	config.DeployedByControllerVersion = s.controllerVersion
	if err := s.db.UpdateServerConfig(ctx, serverID, config); err != nil {
		return fmt.Errorf("failed to update server config: %w", err)
	}
	return nil
}

func (s *ProvisioningService) updateStatus(ctx context.Context, serverID string, status *db.ProvisioningStatus) {
	if err := s.db.UpdateProvisioningStatus(ctx, serverID, status); err != nil {
		log.Printf("Failed to update provisioning status: %v", err)
	}
}

func (s *ProvisioningService) updateStep(ctx context.Context, status *db.ProvisioningStatus, serverID, stepName, message string) {
	for i, step := range status.Steps {
		if step.Name == stepName {
			status.Steps[i].Status = db.ProvisioningStateInProgress
			status.Steps[i].Message = message
			status.Steps[i].Timestamp = time.Now()
			break
		}
	}
	status.CurrentStep = stepName
	s.updateStatus(ctx, serverID, status)
	log.Printf("[%s] Step: %s - %s", serverID, stepName, message)
}

func (s *ProvisioningService) completeStep(ctx context.Context, status *db.ProvisioningStatus, serverID, stepName string) {
	now := time.Now()
	for i, step := range status.Steps {
		if step.Name == stepName {
			status.Steps[i].Status = db.ProvisioningStateCompleted
			status.Steps[i].Message = "Completed"
			status.Steps[i].Timestamp = now
			break
		}
	}
	status.CurrentStep = stepName
	s.updateStatus(ctx, serverID, status)
	log.Printf("[%s] Completed step: %s", serverID, stepName)
}

func (s *ProvisioningService) handleError(status *db.ProvisioningStatus, ctx context.Context, serverID, stepName string, err error) error {
	now := time.Now()
	for i, step := range status.Steps {
		if step.Name == stepName {
			status.Steps[i].Status = db.ProvisioningStateFailed
			status.Steps[i].Message = err.Error()
			status.Steps[i].Timestamp = now
			break
		}
	}
	status.Error = err.Error()
	status.State = db.ProvisioningStateFailed
	status.CurrentStep = stepName
	s.updateStatus(ctx, serverID, status)
	return err
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"i/o timeout",
		"temporary failure",
		"rate limit",
		"quota exceeded",
		"service unavailable",
		"502",
		"503",
		"504",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
