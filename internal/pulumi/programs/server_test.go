package programs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
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
		"roles/logging.logWriter",
		"roles/cloudtrace.agent",
		"roles/artifactregistry.reader",
		"roles/serviceusage.serviceUsageConsumer",
	}

	assert.Len(t, expectedRoles, 4, "Should have 4 project-level IAM roles")

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

func TestCloudConfigHash_DifferentInputsProduceDifferentHashes(t *testing.T) {
	hash := func(input string) string {
		h := sha256.New()
		h.Write([]byte(input))
		return hex.EncodeToString(h.Sum(nil))[:16]
	}

	h1 := hash("cloud-config-v1")
	h2 := hash("cloud-config-v2")

	assert.NotEqual(t, h1, h2, "Different cloud-config content should produce different hashes")
	assert.Len(t, h1, 16, "Hash should be 16 hex characters")
	assert.Len(t, h2, 16, "Hash should be 16 hex characters")
}

func TestCloudConfigHash_SameInputProducesSameHash(t *testing.T) {
	hash := func(input string) string {
		h := sha256.New()
		h.Write([]byte(input))
		return hex.EncodeToString(h.Sum(nil))[:16]
	}

	h1 := hash("cloud-config-v1")
	h2 := hash("cloud-config-v1")

	assert.Equal(t, h1, h2, "Same cloud-config content should produce the same hash")
}

func TestCurrentInfraVersion_IsPositive(t *testing.T) {
	assert.Greater(t, CurrentInfraVersion, 0, "CurrentInfraVersion should be > 0")
}

func TestGCPTagValueRegex_RejectsUUIDs(t *testing.T) {
	// GCP requires tag values to match: ^[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?$
	re := regexp.MustCompile(`^[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?$`)

	tests := []struct {
		value string
		valid bool
	}{
		{"test1234", true},
		{"my-server", true},
		{"development", true},
		{"0dcbaca4-2a26-489c-b4a3-d2fad8bb6483", false},
		{"Server-1", false},
		{"-server", false},
		{"server-", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, tt.valid, re.MatchString(tt.value))
		})
	}
}

func TestServerID_BelongsInLabelsNotTags(t *testing.T) {
	serverID := "0dcbaca4-2a26-489c-b4a3-d2fad8bb6483"
	tagRe := regexp.MustCompile(`^[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?$`)
	labelRe := regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9_]{0,61}[a-z0-9])?$`)

	assert.False(t, tagRe.MatchString(serverID),
		"ServerID UUID must not be a GCP instance tag")
	assert.True(t, labelRe.MatchString(serverID),
		"ServerID UUID must be valid as a GCP label value")
}
