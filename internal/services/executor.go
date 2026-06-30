package services

import (
	"context"
	"time"

	"github.com/nbyl/metio/internal/db"
)

type OperationExecutor interface {
	StartOperation(ctx context.Context, serverID string, opType db.ProvisioningOperation, fn func(context.Context, *db.ProvisioningStatus) error) error
	CancelOperation(ctx context.Context, serverID string) error
	OperationTimeout() time.Duration
}
