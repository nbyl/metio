package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/pulumi"
	"github.com/nbyl/metio/internal/pulumi/programs"
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
	workspaceManager  pulumi.WorkspaceManagerInterface
	db                db.DB
	backupCoord       *BackupCoordinator
	controllerVersion string
	executor          OperationExecutor
	retryAttempts     int
	retryDelay        time.Duration
}

func (s *ProvisioningService) OperationTimeout() time.Duration {
	return s.executor.OperationTimeout()
}

func NewProvisioningService(workspaceManager pulumi.WorkspaceManagerInterface, dbConn db.DB, controllerVersion string, executor OperationExecutor) *ProvisioningService {
	return &ProvisioningService{
		workspaceManager:  workspaceManager,
		db:                dbConn,
		backupCoord:       NewBackupCoordinator(dbConn),
		controllerVersion: controllerVersion,
		executor:          executor,
		retryAttempts:     3,
		retryDelay:        5 * time.Second,
	}
}

func (s *ProvisioningService) CreateServer(ctx context.Context, serverID string, config *programs.ServerConfig) error {
	return s.executor.StartOperation(ctx, serverID, db.ProvisioningOperationCreate, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		return s.runCreate(opCtx, status, serverID, config)
	})
}

func (s *ProvisioningService) UpdateServer(ctx context.Context, serverID string, config *programs.ServerConfig, updateType int) error {
	return s.executor.StartOperation(ctx, serverID, db.ProvisioningOperationUpdate, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		return s.runUpdate(opCtx, status, serverID, config, updateType)
	})
}

func (s *ProvisioningService) DestroyServer(ctx context.Context, serverID string) error {
	return s.executor.StartOperation(ctx, serverID, db.ProvisioningOperationDestroy, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		return s.runDestroy(opCtx, status, serverID)
	})
}

func (s *ProvisioningService) runCreate(opCtx context.Context, status *db.ProvisioningStatus, serverID string, config *programs.ServerConfig) error {
	status.Steps = []db.ProvisioningStep{
		{Name: stepUpsertStack, Status: db.ProvisioningStatePending, Message: "Preparing Pulumi stack...", Timestamp: time.Now()},
		{Name: stepDeployInfrastructure, Status: db.ProvisioningStatePending, Message: "Deploying infrastructure with Pulumi...", Timestamp: time.Now()},
	}
	s.updateStatus(opCtx, serverID, status)

	stack, err := s.workspaceManager.UpsertStack(opCtx, serverID, programs.ServerProgram(config))
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
	result, err := s.backupCoord.WaitForCommandAck(opCtx, config.Name, 60*time.Second)
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
	stack, err := s.workspaceManager.UpsertStack(opCtx, serverID, programs.ServerProgram(config))
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepUpsertStack, err)
	}

	if err := s.workspaceManager.SetConfig(opCtx, stack, "gcp:project", s.workspaceManager.ProjectID(), false); err != nil {
		return s.handleError(status, opCtx, serverID, stepUpsertStack, err)
	}

	s.completeStep(opCtx, status, serverID, stepUpsertStack)

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

	if err := s.db.DeleteServerConfig(opCtx, serverID); err != nil {
		log.Printf("Failed to delete server config for %s: %v", serverID, err)
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
		return s.runCreate(ctx, status, serverID, config)
	case db.ProvisioningOperationUpdate:
		return s.runUpdate(ctx, status, serverID, config, updateType)
	case db.ProvisioningOperationDestroy:
		return s.runDestroy(ctx, status, serverID)
	default:
		return fmt.Errorf("unknown operation type: %v", status.Operation)
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
