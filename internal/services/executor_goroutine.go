package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nbyl/metio/internal/db"
)

type goroutineExecutor struct {
	operations       map[string]context.CancelFunc
	mu               sync.RWMutex
	operationTimeout time.Duration
}

func NewGoroutineExecutor(timeout time.Duration) OperationExecutor {
	return &goroutineExecutor{
		operations:       make(map[string]context.CancelFunc),
		operationTimeout: timeout,
	}
}

func (e *goroutineExecutor) StartOperation(ctx context.Context, serverID string, opType db.ProvisioningOperation, fn func(context.Context, *db.ProvisioningStatus) error) error {
	e.mu.Lock()

	select {
	case <-ctx.Done():
		e.mu.Unlock()
		return ctx.Err()
	default:
	}

	if _, exists := e.operations[serverID]; exists {
		e.mu.Unlock()
		return fmt.Errorf(errMsgOperationInProgress, serverID)
	}

	opCtx, cancel := context.WithTimeout(context.Background(), e.operationTimeout)
	e.operations[serverID] = cancel
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.operations, serverID)
			cancel()
			e.mu.Unlock()
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
		}
	}()

	return nil
}

func (e *goroutineExecutor) CancelOperation(ctx context.Context, serverID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cancel, exists := e.operations[serverID]
	if !exists {
		return fmt.Errorf(errMsgNoOperationInProgress, serverID)
	}

	cancel()
	delete(e.operations, serverID)

	return nil
}

func (e *goroutineExecutor) OperationTimeout() time.Duration {
	return e.operationTimeout
}
