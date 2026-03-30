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
	return s.queueOperation(ctx, serverID, db.OperationTypeCreate, func(opCtx context.Context, op *db.Operation) error {
		op.Steps = []db.OperationStep{
			{Name: stepCreateServiceAccount, Description: "Creating service account...", Completed: false},
			{Name: stepCreateBackupBucket, Description: "Creating backup bucket...", Completed: false},
			{Name: stepReserveStaticIP, Description: "Reserving static IP...", Completed: false},
			{Name: stepCreateDisk, Description: "Creating disk...", Completed: false},
			{Name: stepCreateFirewall, Description: "Creating firewall rules...", Completed: false},
			{Name: stepCreateInstance, Description: "Creating VM instance...", Completed: false},
			{Name: stepDeployInfrastructure, Description: "Deploying infrastructure with Pulumi...", Completed: false},
		}
		s.updateOperation(opCtx, serverID, op)

		config.Name = serverID

		stack, err := s.workspaceManager.UpsertStack(opCtx, serverID, programs.ServerProgram(config))
		if err != nil {
			return s.handleError(op, opCtx, serverID, stepUpsertStack, err)
		}

		s.completeStep(op, serverID, stepCreateServiceAccount)

		s.updateStep(opCtx, serverID, stepDeployInfrastructure, "Deploying infrastructure with Pulumi...")
		result, err := s.executeUpWithRetry(opCtx, func() (auto.UpResult, error) {
			return s.workspaceManager.UpStack(opCtx, stack)
		})
		if err != nil {
			return s.handleError(op, opCtx, serverID, stepDeployInfrastructure, err)
		}

		s.completeStep(op, serverID, stepDeployInfrastructure)

		outputs := make(map[string]string)
		for key, value := range result.Outputs {
			if str, ok := value.Value.(string); ok {
				outputs[key] = str
			}
		}

		op.Outputs = outputs
		op.State = db.OperationStateCompleted
		s.updateOperation(opCtx, serverID, op)

		return nil
	})
}

func (s *ProvisioningService) UpdateServer(ctx context.Context, serverID string, config *programs.ServerConfig) error {
	return s.queueOperation(ctx, serverID, db.OperationTypeUpdate, func(opCtx context.Context, op *db.Operation) error {
		op.Steps = []db.OperationStep{
			{Name: stepRefreshStack, Description: "Refreshing Pulumi stack...", Completed: false},
			{Name: stepUpdateInfrastructure, Description: "Updating infrastructure...", Completed: false},
		}
		s.updateOperation(opCtx, serverID, op)

		config.Name = serverID

		stack, err := s.workspaceManager.UpsertStack(opCtx, serverID, programs.ServerProgram(config))
		if err != nil {
			return s.handleError(op, opCtx, serverID, stepUpsertStack, err)
		}

		s.completeStep(op, serverID, stepRefreshStack)

		s.updateStep(opCtx, serverID, stepUpdateInfrastructure, "Updating infrastructure...")
		result, err := s.executeUpWithRetry(opCtx, func() (auto.UpResult, error) {
			return s.workspaceManager.UpStack(opCtx, stack)
		})
		if err != nil {
			return s.handleError(op, opCtx, serverID, stepUpdateInfrastructure, err)
		}

		s.completeStep(op, serverID, stepUpdateInfrastructure)

		outputs := make(map[string]string)
		for key, value := range result.Outputs {
			if str, ok := value.Value.(string); ok {
				outputs[key] = str
			}
		}

		op.Outputs = outputs
		op.State = db.OperationStateCompleted
		s.updateOperation(opCtx, serverID, op)

		return nil
	})
}

func (s *ProvisioningService) DestroyServer(ctx context.Context, serverID string) error {
	return s.queueOperation(ctx, serverID, db.OperationTypeDelete, func(opCtx context.Context, op *db.Operation) error {
		op.Steps = []db.OperationStep{
			{Name: stepDestroyStack, Description: "Destroying Pulumi stack...", Completed: false},
			{Name: stepCleanupResources, Description: "Cleaning up resources...", Completed: false},
		}
		s.updateOperation(opCtx, serverID, op)

		s.updateStep(opCtx, serverID, stepDestroyStack, "Destroying Pulumi stack...")
		err := s.executeWithRetry(opCtx, func() error {
			return s.workspaceManager.DestroyStack(opCtx, serverID)
		})
		if err != nil {
			return s.handleError(op, opCtx, serverID, stepDestroyStack, err)
		}

		s.completeStep(op, serverID, stepDestroyStack)
		s.completeStep(op, serverID, stepCleanupResources)

		op.State = db.OperationStateCompleted
		s.updateOperation(opCtx, serverID, op)

		return nil
	})
}

func (s *ProvisioningService) queueOperation(ctx context.Context, serverID string, opType db.OperationType, fn func(context.Context, *db.Operation) error) error {
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

		op := &db.Operation{
			ID:          fmt.Sprintf("%s-%d", serverID, time.Now().Unix()),
			Type:        opType,
			State:       db.OperationStateRunning,
			CurrentStep: "initializing",
			Steps:       []db.OperationStep{},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := fn(opCtx, op); err != nil {
			if opCtx.Err() == context.Canceled {
				op.State = db.OperationStateCancelled
			} else {
				op.State = db.OperationStateFailed
				op.Error = err.Error()
			}
			s.updateOperation(opCtx, serverID, op)
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

func (s *ProvisioningService) GetOperationStatus(ctx context.Context, serverID string) (*db.Operation, error) {
	return s.db.GetOperation(ctx, serverID)
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

func (s *ProvisioningService) updateOperation(ctx context.Context, serverID string, op *db.Operation) {
	op.UpdatedAt = time.Now()
	if err := s.db.UpdateOperation(ctx, serverID, op); err != nil {
		log.Printf("Failed to update operation: %v", err)
	}
}

func (s *ProvisioningService) updateStep(ctx context.Context, serverID, stepName, description string) {
	log.Printf("[%s] Step: %s - %s", serverID, stepName, description)
}

func (s *ProvisioningService) completeStep(op *db.Operation, serverID, stepName string) {
	for i, step := range op.Steps {
		if step.Name == stepName {
			op.Steps[i].Completed = true
			break
		}
	}
	op.CurrentStep = stepName
	log.Printf("[%s] Completed step: %s", serverID, stepName)
}

func (s *ProvisioningService) handleError(op *db.Operation, ctx context.Context, serverID, stepName string, err error) error {
	for i, step := range op.Steps {
		if step.Name == stepName {
			op.Steps[i].Error = err.Error()
			break
		}
	}
	op.Error = err.Error()
	op.State = db.OperationStateFailed
	op.CurrentStep = stepName
	s.updateOperation(ctx, serverID, op)
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
