package setup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/nbyl/metio/internal/services"
)

type mockValidationService struct {
	validateFunc func(ctx context.Context) (*services.ValidationResult, error)
}

func (m *mockValidationService) Validate(ctx context.Context) (*services.ValidationResult, error) {
	return m.validateFunc(ctx)
}

func TestValidateSetupHandler_Valid(t *testing.T) {
	original := ValidationService
	defer func() { ValidationService = original }()

	ValidationService = &mockValidationService{
		validateFunc: func(ctx context.Context) (*services.ValidationResult, error) {
			return &services.ValidationResult{
				Valid:       true,
				APIs:        map[string]services.APICheckResult{"compute.googleapis.com": {Enabled: true}},
				Permissions: map[string]services.PermissionCheckResult{"compute.instances.create": {Granted: true}},
				Fixes:       []services.Fix{},
			}, nil
		},
	}

	req := httptest.NewRequest("GET", "/api/setup/validate", nil)
	w := httptest.NewRecorder()

	ValidateSetupHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	var result services.ValidationResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Fixes)
}

func TestValidateSetupHandler_InvalidWithFixes(t *testing.T) {
	original := ValidationService
	defer func() { ValidationService = original }()

	ValidationService = &mockValidationService{
		validateFunc: func(ctx context.Context) (*services.ValidationResult, error) {
			return &services.ValidationResult{
				Valid: false,
				Fixes: []services.Fix{
					{Type: "enable_api", API: "iam.googleapis.com", ConsoleURL: "https://console.cloud.google.com/apis/library/iam.googleapis.com"},
					{Type: "grant_role", Role: "roles/compute.admin", Permission: "compute.instances.create", ConsoleURL: "https://console.cloud.google.com/iam-admin/iam"},
				},
			}, nil
		},
	}

	req := httptest.NewRequest("GET", "/api/setup/validate", nil)
	w := httptest.NewRecorder()

	ValidateSetupHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result services.ValidationResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Len(t, result.Fixes, 2)
	assert.Equal(t, "enable_api", result.Fixes[0].Type)
	assert.Equal(t, "grant_role", result.Fixes[1].Type)
}

func TestValidateSetupHandler_ServiceError(t *testing.T) {
	original := ValidationService
	defer func() { ValidationService = original }()

	ValidationService = &mockValidationService{
		validateFunc: func(ctx context.Context) (*services.ValidationResult, error) {
			return nil, errors.New("gcp api error")
		},
	}

	req := httptest.NewRequest("GET", "/api/setup/validate", nil)
	w := httptest.NewRecorder()

	ValidateSetupHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errResp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "validation failed", errResp["error"])
}
