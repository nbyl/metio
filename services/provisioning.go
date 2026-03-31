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
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/pulumi"
	"gitlab.com/nbyl/metio/pulumi/programs"
)

const (
	stepCreateServiceAccount = "create_service_account"
	stepCreateBackupBucket   = "create_backup_bucket"
	stepReserveStaticIP      = "reserve_static_ip"
	stepCreateDisk           = "create_disk"
	stepCreateFirewall       = "create_firewall"
	stepCreateInstance       = "create_instance"
	stepDeployInfrastructure = "deploy_infrastructure"
	stepRefreshStack         = "refresh_stack"
	stepUpdateInfrastructure = "update_infrastructure"
	stepDestroyStack         = "destroy_stack"
	stepCleanupResources     = "cleanup_resources"
	stepUpsertStack          = "upsert_stack"
)

const errMsgOperationInProgress = "operation already in progress for server %s"
const errMsgNoOperationInProgress = "no operation in progress for server %s"
const errMsgRetryExhausted = "operation failed after %d attempts: %w"

var errOperationInProgress = errors.New("operation already in progress")
var errNoOperationInProgress = errors.New("no operation in progress")

type ProvisioningService struct {
	workspaceManager *pulumi.WorkspaceManager
	db               db.DB
	operations       map[string]context.CancelFunc
	mu               sync.RWMutex
	operationTimeout time.Duration
	retryAttempts    int
	retryDelay       time.Duration
}

func NewProvisioningService(workspaceManager *pulumi.WorkspaceManager, dbConn db.DB) *ProvisioningService {
	return &ProvisioningService{
		workspaceManager: workspaceManager,
		db:               dbConn,
		operations:       make(map[string]context.CancelFunc),
		operationTimeout: 30 * time.Minute,
		retryAttempts:    3,
		retryDelay:       5 * time.Second,
	}
}

func (s *ProvisioningService) CreateServer(ctx context.Context, serverID string, config *programs.ServerConfig) error {
	return s.queueOperation(ctx, serverID, db.ProvisioningOperationCreate, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		status.Steps = []db.ProvisioningStep{
			{Name: stepCreateServiceAccount, Status: db.ProvisioningStatePending, Message: "Creating service account...", Timestamp: time.Now()},
			{Name: stepCreateBackupBucket, Status: db.ProvisioningStatePending, Message: "Creating backup bucket...", Timestamp: time.Now()},
			{Name: stepReserveStaticIP, Status: db.ProvisioningStatePending, Message: "Reserving static IP...", Timestamp: time.Now()},
			{Name: stepCreateDisk, Status: db.ProvisioningStatePending, Message: "Creating disk...", Timestamp: time.Now()},
			{Name: stepCreateFirewall, Status: db.ProvisioningStatePending, Message: "Creating firewall rules...", Timestamp: time.Now()},
			{Name: stepCreateInstance, Status: db.ProvisioningStatePending, Message: "Creating VM instance...", Timestamp: time.Now()},
			{Name: stepDeployInfrastructure, Status: db.ProvisioningStatePending, Message: "Deploying infrastructure with Pulumi...", Timestamp: time.Now()},
		}
		s.updateStatus(opCtx, serverID, status)

		config.Name = serverID

		stack, err := s.workspaceManager.UpsertStack(opCtx, serverID, programs.ServerProgram(config))
		if err != nil {
			return s.handleError(status, opCtx, serverID, stepUpsertStack, err)
		}

		s.completeStep(status, serverID, stepCreateServiceAccount)

		s.updateStep(opCtx, serverID, stepDeployInfrastructure, "Deploying infrastructure with Pulumi...")
		result, err := s.executeUpWithRetry(opCtx, func() (auto.UpResult, error) {
			return s.workspaceManager.UpStack(opCtx, stack)
		})
		if err != nil {
			return s.handleError(status, opCtx, serverID, stepDeployInfrastructure, err)
		}

		s.completeStep(status, serverID, stepDeployInfrastructure)

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

func (s *ProvisioningService) UpdateServer(ctx context.Context, serverID string, config *programs.ServerConfig) error {
	return s.queueOperation(ctx, serverID, db.ProvisioningOperationUpdate, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		status.Steps = []db.ProvisioningStep{
			{Name: stepRefreshStack, Status: db.ProvisioningStatePending, Message: "Refreshing Pulumi stack...", Timestamp: time.Now()},
			{Name: stepUpdateInfrastructure, Status: db.ProvisioningStatePending, Message: "Updating infrastructure...", Timestamp: time.Now()},
		}
		s.updateStatus(opCtx, serverID, status)

		config.Name = serverID

		stack, err := s.workspaceManager.UpsertStack(opCtx, serverID, programs.ServerProgram(config))
		if err != nil {
			return s.handleError(status, opCtx, serverID, stepUpsertStack, err)
		}

		s.completeStep(status, serverID, stepRefreshStack)

		s.updateStep(opCtx, serverID, stepUpdateInfrastructure, "Updating infrastructure...")
		result, err := s.executeUpWithRetry(opCtx, func() (auto.UpResult, error) {
			return s.workspaceManager.UpStack(opCtx, stack)
		})
		if err != nil {
			return s.handleError(status, opCtx, serverID, stepUpdateInfrastructure, err)
		}

		s.completeStep(status, serverID, stepUpdateInfrastructure)

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

func (s *ProvisioningService) DestroyServer(ctx context.Context, serverID string) error {
	return s.queueOperation(ctx, serverID, db.ProvisioningOperationDestroy, func(opCtx context.Context, status *db.ProvisioningStatus) error {
		status.Steps = []db.ProvisioningStep{
			{Name: stepDestroyStack, Status: db.ProvisioningStatePending, Message: "Destroying Pulumi stack...", Timestamp: time.Now()},
			{Name: stepCleanupResources, Status: db.ProvisioningStatePending, Message: "Cleaning up resources...", Timestamp: time.Now()},
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
		s.completeStep(status, serverID, stepCleanupResources)

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

	opCtx, cancel := context.WithTimeout(ctx, s.operationTimeout)
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
