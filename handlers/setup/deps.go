package setup

import (
	"context"

	"gitlab.com/nbyl/metio/services"
)

type ValidationServiceInterface interface {
	Validate(ctx context.Context) (*services.ValidationResult, error)
}

var ValidationService ValidationServiceInterface
