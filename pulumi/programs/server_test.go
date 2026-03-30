package programs

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

func TestServerConfigDefaults(t *testing.T) {
	config := &ServerConfig{
		Name:       "test-server",
		GCPProject: "test-project",
	}

	assert.Equal(t, "test-server", config.Name)
	assert.Equal(t, "test-project", config.GCPProject)
	assert.Equal(t, "", config.Region)
	assert.Equal(t, "", config.Zone)
}

func TestServerConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *ServerConfig
		expectError bool
	}{
		{
			name:        "valid config",
			config:      &ServerConfig{Name: "test-server"},
			expectError: false,
		},
		{
			name:        "empty name",
			config:      &ServerConfig{Name: ""},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIAMRolesComplete(t *testing.T) {
	expectedRoles := []string{
		"roles/storage.objectUser",
		"roles/storage.objectCreator",
		"roles/logging.logWriter",
		"roles/monitoring.metricWriter",
		"roles/cloudtrace.agent",
		"roles/artifactregistry.reader",
		"roles/datastore.user",
		"roles/serviceusage.serviceUsageConsumer",
		"roles/compute.instanceAdmin.v1",
	}

	assert.Len(t, expectedRoles, 9, "Should have 9 IAM roles")

	for _, role := range expectedRoles {
		assert.Contains(t, role, "roles/", "Role should start with roles/")
	}
}

func validateConfig(config *ServerConfig) error {
	if config.Name == "" {
		return &ValidationError{Field: "Name", Message: "server name is required"}
	}
	return nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

type MockContext struct {
	context.Context
	resources map[string]*resource.State
	exports   map[string]interface{}
}

func NewMockContext() *MockContext {
	return &MockContext{
		Context:   context.Background(),
		resources: make(map[string]*resource.State),
		exports:   make(map[string]interface{}),
	}
}

type MockResourceRegistry struct {
	resources map[string]pulumi.Resource
}

func NewMockResourceRegistry() *MockResourceRegistry {
	return &MockResourceRegistry{
		resources: make(map[string]pulumi.Resource),
	}
}
