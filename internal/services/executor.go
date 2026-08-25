package services

import (
	"context"
	"time"

	"github.com/nbyl/metio/internal/db"
)

type OperationExecutor interface {
	StartOperation(ctx context.Context, serverID string, opType db.ProvisioningOperation, initialOutputs map[string]string, fn func(context.Context, *db.ProvisioningStatus) error) error
	CancelOperation(ctx context.Context, serverID string) error
	OperationTimeout() time.Duration
}
