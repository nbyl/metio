package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any env vars that might be set
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("REGION")
	os.Unsetenv("INSTANCE_NAME")
	os.Unsetenv("GCP_PROJECT")
	viper.Reset()
	viper.AutomaticEnv()

	cfg := Load()

	assert.Equal(t, DefaultEnvironment, cfg.Environment)
	assert.Equal(t, DefaultRegion, cfg.Region)
	assert.Equal(t, DefaultInstanceName, cfg.InstanceName)
	assert.Equal(t, "", cfg.ProjectID)
}

func TestLoad_WithEnvVars(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("REGION", "europe-west1")
	t.Setenv("INSTANCE_NAME", "my-server")
	t.Setenv("GCP_PROJECT", "my-project")

	cfg := Load()

	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, "europe-west1", cfg.Region)
	assert.Equal(t, "my-server", cfg.InstanceName)
	assert.Equal(t, "my-project", cfg.ProjectID)
}

func TestLoad_PartialEnvVars(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("REGION")
	t.Setenv("INSTANCE_NAME", "custom-server")
	t.Setenv("GCP_PROJECT", "proj-123")

	cfg := Load()

	assert.Equal(t, DefaultEnvironment, cfg.Environment)
	assert.Equal(t, DefaultRegion, cfg.Region)
	assert.Equal(t, "custom-server", cfg.InstanceName)
	assert.Equal(t, "proj-123", cfg.ProjectID)
}

func TestDatabaseID(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "default config",
			config: Config{
				Environment: "development",
				Region:      "us-central1",
			},
			expected: "development-us-central1-metio-db",
		},
		{
			name: "production config",
			config: Config{
				Environment: "production",
				Region:      "europe-west1",
			},
			expected: "production-europe-west1-metio-db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.DatabaseID())
		})
	}
}

func TestLoadWithMetadata_FailsOutsideGCE(t *testing.T) {
	// Outside GCE, metadata.ProjectID() should fail
	viper.Reset()
	viper.AutomaticEnv()

	_, err := LoadWithMetadata()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project ID from metadata")
}
