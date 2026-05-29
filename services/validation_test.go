package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/serviceusage/v1"
)

type mockServiceUsageClient struct {
	getServiceFunc  func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error)
	getServiceCalls int
}

func (m *mockServiceUsageClient) GetService(ctx context.Context, name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
	m.getServiceCalls++
	return m.getServiceFunc(name)
}

func (m *mockServiceUsageClient) Close() error { return nil }

type mockResourceManagerClient struct {
	testIamPermissionsFunc  func(projectID string, permissions []string) ([]string, error)
	testIamPermissionsCalls int
}

func (m *mockResourceManagerClient) TestIamPermissions(ctx context.Context, projectID string, permissions []string) ([]string, error) {
	m.testIamPermissionsCalls++
	return m.testIamPermissionsFunc(projectID, permissions)
}

func (m *mockResourceManagerClient) Close() error { return nil }

func newTestValidationService(su ServiceUsageClient, rm ResourceManagerClient) *ValidationService {
	return &ValidationService{
		projectID:       "test-project",
		serviceUsage:    su,
		resourceManager: rm,
		cache:           newValidationCache(5 * time.Minute),
	}
}

func TestValidate_AllEnabledAllGranted(t *testing.T) {
	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			return permissions, nil
		},
	}

	svc := newTestValidationService(su, rm)
	result, err := svc.Validate(context.Background())

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Fixes)
	assert.Len(t, result.APIs, len(requiredAPIs))
	assert.Len(t, result.Permissions, len(requiredPermissions))

	for api, apiResult := range result.APIs {
		assert.True(t, apiResult.Enabled, "API %s should be enabled", api)
	}
	for perm, permResult := range result.Permissions {
		assert.True(t, permResult.Granted, "Permission %s should be granted", perm)
	}
}

func TestValidate_OneAPIDisabled(t *testing.T) {
	disabledAPI := "iam.googleapis.com"
	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			if name == "projects/test-project/services/"+disabledAPI {
				return &serviceusage.GoogleApiServiceusageV1Service{State: "DISABLED"}, nil
			}
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			return permissions, nil
		},
	}

	svc := newTestValidationService(su, rm)
	result, err := svc.Validate(context.Background())

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Len(t, result.Fixes, 1)
	assert.Equal(t, "enable_api", result.Fixes[0].Type)
	assert.Equal(t, disabledAPI, result.Fixes[0].API)
	assert.Contains(t, result.Fixes[0].ConsoleURL, disabledAPI)
	assert.Contains(t, result.Fixes[0].ConsoleURL, "test-project")
}

func TestValidate_OnePermissionMissing(t *testing.T) {
	missingPerm := "compute.instances.create"
	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			var granted []string
			for _, p := range permissions {
				if p != missingPerm {
					granted = append(granted, p)
				}
			}
			return granted, nil
		},
	}

	svc := newTestValidationService(su, rm)
	result, err := svc.Validate(context.Background())

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Len(t, result.Fixes, 1)
	assert.Equal(t, "grant_role", result.Fixes[0].Type)
	assert.Equal(t, missingPerm, result.Fixes[0].Permission)
	assert.Equal(t, "roles/compute.admin", result.Fixes[0].Role)
	assert.Contains(t, result.Fixes[0].ConsoleURL, "iam-admin")
}

func TestValidate_MixedFailures(t *testing.T) {
	disabledAPIs := map[string]bool{"iam.googleapis.com": true, "run.googleapis.com": true}
	missingPerms := map[string]bool{
		"compute.instances.create":   true,
		"storage.buckets.create":     true,
		"iam.serviceAccounts.create": true,
	}

	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			for api := range disabledAPIs {
				if name == "projects/test-project/services/"+api {
					return &serviceusage.GoogleApiServiceusageV1Service{State: "DISABLED"}, nil
				}
			}
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			var granted []string
			for _, p := range permissions {
				if !missingPerms[p] {
					granted = append(granted, p)
				}
			}
			return granted, nil
		},
	}

	svc := newTestValidationService(su, rm)
	result, err := svc.Validate(context.Background())

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Len(t, result.Fixes, 5)

	apiFixCount := 0
	permFixCount := 0
	for _, f := range result.Fixes {
		if f.Type == "enable_api" {
			apiFixCount++
			assert.True(t, disabledAPIs[f.API])
		}
		if f.Type == "grant_role" {
			permFixCount++
			assert.True(t, missingPerms[f.Permission])
		}
	}
	assert.Equal(t, 2, apiFixCount)
	assert.Equal(t, 3, permFixCount)
}

func TestValidate_ServiceUsageError(t *testing.T) {
	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			return nil, errors.New("network error")
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			return permissions, nil
		},
	}

	svc := newTestValidationService(su, rm)
	result, err := svc.Validate(context.Background())

	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Len(t, result.Fixes, len(requiredAPIs))
}

func TestValidate_TestIamPermissionsError(t *testing.T) {
	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			return nil, errors.New("permission denied")
		},
	}

	svc := newTestValidationService(su, rm)
	_, err := svc.Validate(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestValidate_CacheHit(t *testing.T) {
	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			return permissions, nil
		},
	}

	svc := newTestValidationService(su, rm)

	_, err := svc.Validate(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(requiredAPIs), su.getServiceCalls)
	assert.Equal(t, 1, rm.testIamPermissionsCalls)

	_, err = svc.Validate(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(requiredAPIs), su.getServiceCalls, "should not call GetService again on cache hit")
	assert.Equal(t, 1, rm.testIamPermissionsCalls, "should not call TestIamPermissions again on cache hit")
}

func TestValidate_CacheMissAfterExpiry(t *testing.T) {
	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			return permissions, nil
		},
	}

	svc := &ValidationService{
		projectID:       "test-project",
		serviceUsage:    su,
		resourceManager: rm,
		cache:           newValidationCache(10 * time.Millisecond),
	}

	_, err := svc.Validate(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(requiredAPIs), su.getServiceCalls)
	assert.Equal(t, 1, rm.testIamPermissionsCalls)

	time.Sleep(20 * time.Millisecond)

	_, err = svc.Validate(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2*len(requiredAPIs), su.getServiceCalls, "should call GetService again after cache expiry")
	assert.Equal(t, 2, rm.testIamPermissionsCalls, "should call TestIamPermissions again after cache expiry")
}

func TestValidate_UnmappedPermission(t *testing.T) {
	originalMap := permissionToRole
	permissionToRole = map[string]string{}
	defer func() { permissionToRole = originalMap }()

	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			var granted []string
			for _, p := range permissions {
				if p != "compute.instances.create" {
					granted = append(granted, p)
				}
			}
			return granted, nil
		},
	}

	svc := newTestValidationService(su, rm)
	result, err := svc.Validate(context.Background())

	require.NoError(t, err)
	assert.False(t, result.Valid)

	var fix *Fix
	for i, f := range result.Fixes {
		if f.Permission == "compute.instances.create" {
			fix = &result.Fixes[i]
			break
		}
	}
	require.NotNil(t, fix)
	assert.Equal(t, "roles/owner", fix.Role)
}

func TestValidate_AllPermissionsMissing(t *testing.T) {
	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			return []string{}, nil
		},
	}

	svc := newTestValidationService(su, rm)
	result, err := svc.Validate(context.Background())

	require.NoError(t, err)
	assert.False(t, result.Valid)

	permFixCount := 0
	for _, f := range result.Fixes {
		if f.Type == "grant_role" {
			permFixCount++
		}
	}
	assert.Equal(t, len(requiredPermissions), permFixCount)
}

func TestValidate_InvalidateCache(t *testing.T) {
	su := &mockServiceUsageClient{
		getServiceFunc: func(name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
			return &serviceusage.GoogleApiServiceusageV1Service{State: "ENABLED"}, nil
		},
	}
	rm := &mockResourceManagerClient{
		testIamPermissionsFunc: func(projectID string, permissions []string) ([]string, error) {
			return permissions, nil
		},
	}

	svc := newTestValidationService(su, rm)

	_, err := svc.Validate(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(requiredAPIs), su.getServiceCalls)

	svc.InvalidateCache()

	_, err = svc.Validate(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2*len(requiredAPIs), su.getServiceCalls, "should call GetService again after invalidation")
}
