package setup

import (
	"context"

	"github.com/nbyl/metio/internal/services"
)

type ValidationServiceInterface interface {
	Validate(ctx context.Context) (*services.ValidationResult, error)
}

type SetupServiceInterface interface {
	IsInitialized(ctx context.Context) bool
	Initialize(ctx context.Context) error
	ServerCount(ctx context.Context) (int, error)
}

var ValidationService ValidationServiceInterface
var SetupService SetupServiceInterface
