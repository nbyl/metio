package setup

import (
	"context"

	"gitlab.com/nbyl/metio/internal/services"
)

type ValidationServiceInterface interface {
	Validate(ctx context.Context) (*services.ValidationResult, error)
}

var ValidationService ValidationServiceInterface
