package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"gitlab.com/nbyl/metio/internal/db"
	"gitlab.com/nbyl/metio/internal/pulumi"
	"gitlab.com/nbyl/metio/internal/pulumi/programs"
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
	operations        map[string]context.CancelFunc
	mu                sync.RWMutex
	operationTimeout  time.Duration
	retryAttempts     int
	retryDelay        time.Duration
}

func NewProvisioningService(workspaceManager pulumi.WorkspaceManagerInterface, dbConn db.DB, controllerVersion string) *ProvisioningService {
	return &ProvisioningService{
		workspaceManager:  workspaceManager,
		db:                dbConn,
		backupCoord:       NewBackupCoordinator(dbConn),
		controllerVersion: controllerVersion,
		operations:        make(map[string]context.CancelFunc),
		operationTimeout:  30 * time.Minute,
		retryAttempts:     3,
		retryDelay:        5 * time.Second,
	}
}

func (s *ProvisioningService) CreateServer(ctx context.Context, serverID string, config *programs.ServerConfig) error {
	return s.queueOperation(ctx, serverID, db.ProvisioningOperationCreate, func(opCtx context.Context, status *db.ProvisioningStatus) error {
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

		s.completeStep(status, serverID, stepUpsertStack)

		s.updateStep(opCtx, serverID, stepDeployInfrastructure, "Deploying infrastructure with Pulumi...")
		result, err := s.executeUpWithRetry(opCtx, func() (auto.UpResult, error) {
			return s.workspaceManager.UpStack(opCtx, stack)
		})
		if err != nil {
			return s.handleError(status, opCtx, serverID, stepDeployInfrastructure, err)
		}

		s.completeStep(status, serverID, stepDeployInfrastructure)

		// Stamp the server config with the current infrastructure and controller versions.
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
	})
}

func (s *ProvisioningService) UpdateServer(ctx context.Context, serverID string, config *programs.ServerConfig, updateType int) error {
	return s.queueOperation(ctx, serverID, db.ProvisioningOperationUpdate, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		return s.runUpdate(opCtx, status, serverID, config, updateType)
	})
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

	s.updateStep(opCtx, serverID, stepStopInstance, "Stopping VM instance...")
	if err := StopInstance(opCtx, config.GCPProject, config.Zone, config.Name); err != nil {
		return s.handleError(status, opCtx, serverID, stepStopInstance, err)
	}
	s.completeStep(status, serverID, stepStopInstance)

	if err := s.runPulumiUpdate(opCtx, status, serverID, config); err != nil {
		// If pulumi fails after stopping, attempt to start the VM back.
		if startErr := StartInstance(opCtx, config.GCPProject, config.Zone, config.Name); startErr != nil {
			log.Printf("Failed to restart instance %s after update failure: %v", serverID, startErr)
		}
		return err
	}

	s.updateStep(opCtx, serverID, stepStartInstance, "Starting VM instance...")
	if err := StartInstance(opCtx, config.GCPProject, config.Zone, config.Name); err != nil {
		return s.handleError(status, opCtx, serverID, stepStartInstance, err)
	}
	s.completeStep(status, serverID, stepStartInstance)

	s.updateStep(opCtx, serverID, stepHealthCheck, "Waiting for server to become healthy...")
	if err := WaitForServerHealthy(opCtx, s.db, config.Name, 120*time.Second); err != nil {
		log.Printf("Warning: server %s did not become healthy after resize: %v", serverID, err)
	}
	s.completeStep(status, serverID, stepHealthCheck)

	return nil
}

func (s *ProvisioningService) runRecreateUpdate(opCtx context.Context, status *db.ProvisioningStatus, serverID string, config *programs.ServerConfig) error {
	status.Steps = []db.ProvisioningStep{
		{Name: stepSaveWorld, Status: db.ProvisioningStatePending, Message: "Saving world data...", Timestamp: time.Now()},
		{Name: stepUpsertStack, Status: db.ProvisioningStatePending, Message: "Updating infrastructure...", Timestamp: time.Now()},
	}
	s.updateStatus(opCtx, serverID, status)

	s.updateStep(opCtx, serverID, stepSaveWorld, "Saving world data...")
	if err := s.backupCoord.TriggerWorldSave(opCtx, config.Name); err != nil {
		return s.handleError(status, opCtx, serverID, stepSaveWorld,
			fmt.Errorf("failed to trigger world save: %w", err))
	}
	result, err := s.backupCoord.WaitForCommandAck(opCtx, config.Name, 60*time.Second)
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepSaveWorld,
			fmt.Errorf("world save failed: result=%s, %w", result, err))
	}
	s.completeStep(status, serverID, stepSaveWorld)

	if err := s.runPulumiUpdate(opCtx, status, serverID, config); err != nil {
		return err
	}

	s.updateStep(opCtx, serverID, stepHealthCheck, "Waiting for server to become healthy...")
	if err := WaitForServerHealthy(opCtx, s.db, config.Name, 180*time.Second); err != nil {
		log.Printf("Warning: server %s did not become healthy after recreate: %v", serverID, err)
	}
	s.completeStep(status, serverID, stepHealthCheck)

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

	s.completeStep(status, serverID, stepUpsertStack)

	s.updateStep(opCtx, serverID, stepUpdateInfrastructure, "Updating infrastructure...")
	result, err := s.executeUpWithRetry(opCtx, func() (auto.UpResult, error) {
		return s.workspaceManager.UpStack(opCtx, stack)
	})
	if err != nil {
		return s.handleError(status, opCtx, serverID, stepUpdateInfrastructure, err)
	}

	s.completeStep(status, serverID, stepUpdateInfrastructure)

	// Stamp the server config with the current infrastructure and controller versions.
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

// RevertServerConfig replaces the server config with a previously saved snapshot.
// This is called by handlers when an update operation fails.
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

func (s *ProvisioningService) DestroyServer(ctx context.Context, serverID string) error {
	return s.queueOperation(ctx, serverID, db.ProvisioningOperationDestroy, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		status.Steps = []db.ProvisioningStep{
			{Name: stepDestroyStack, Status: db.ProvisioningStatePending, Message: "Destroying Pulumi stack...", Timestamp: time.Now()},
		}
		s.updateStatus(opCtx, serverID, status)

		s.updateStep(opCtx, serverID, stepDestroyStack, "Destroying Pulumi stack...")
		err := s.executeWithRetry(opCtx, func() error {
			return s.workspaceManager.DestroyStack(opCtx, serverID)
		})
		if err != nil {
			return s.handleError(status, opCtx, serverID, stepDestroyStack, err)
		}

		s.completeStep(status, serverID, stepDestroyStack)

		status.State = db.ProvisioningStateCompleted
		s.updateStatus(opCtx, serverID, status)

		return nil
	})
}

func (s *ProvisioningService) queueOperation(ctx context.Context, serverID string, opType db.ProvisioningOperation, fn func(context.Context, *db.ProvisioningStatus) error) error {
	s.mu.Lock()

	select {
	case <-ctx.Done():
		s.mu.Unlock()
		return ctx.Err()
	default:
	}

	if _, exists := s.operations[serverID]; exists {
		s.mu.Unlock()
		return fmt.Errorf(errMsgOperationInProgress, serverID)
	}

	// Use context.Background() so the operation survives HTTP request cancellation.
	// The operation has its own timeout independent of the caller's context.
	opCtx, cancel := context.WithTimeout(context.Background(), s.operationTimeout)
	s.operations[serverID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.operations, serverID)
			cancel()
			s.mu.Unlock()
		}()

		now := time.Now()
		status := &db.ProvisioningStatus{
			ID:          fmt.Sprintf("%s-%d", serverID, now.Unix()),
			Operation:   opType,
			State:       db.ProvisioningStateInProgress,
			StartedAt:   now,
			CurrentStep: "initializing",
			Steps:       []db.ProvisioningStep{},
		}

		if err := fn(opCtx, status); err != nil {
			if opCtx.Err() == context.Canceled {
				status.State = db.ProvisioningStateFailed
			} else {
				status.State = db.ProvisioningStateFailed
				status.Error = err.Error()
			}
			s.updateStatus(opCtx, serverID, status)
		}
	}()

	return nil
}

func (s *ProvisioningService) CancelOperation(ctx context.Context, serverID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, exists := s.operations[serverID]
	if !exists {
		return fmt.Errorf(errMsgNoOperationInProgress, serverID)
	}

	cancel()
	delete(s.operations, serverID)

	return nil
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

// stampServerConfig updates the given server's config with the current infrastructure version
// and the controller version that deployed it.
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

func (s *ProvisioningService) updateStep(ctx context.Context, serverID, stepName, message string) {
	log.Printf("[%s] Step: %s - %s", serverID, stepName, message)
}

func (s *ProvisioningService) completeStep(status *db.ProvisioningStatus, serverID, stepName string) {
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
