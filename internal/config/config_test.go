package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoad_MissingMachineAgentImage(t *testing.T) {
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("REGION")
	os.Unsetenv("GCP_PROJECT")
	os.Unsetenv("MACHINE_AGENT_IMAGE")
	viper.Reset()
	viper.AutomaticEnv()

	cfg, err := Load()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MACHINE_AGENT_IMAGE must be set")
	assert.Equal(t, DefaultEnvironment, cfg.Environment)
	assert.Equal(t, DefaultRegion, cfg.Region)
}

func TestLoad_Defaults(t *testing.T) {
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("REGION")
	os.Unsetenv("GCP_PROJECT")
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, DefaultEnvironment, cfg.Environment)
	assert.Equal(t, DefaultRegion, cfg.Region)
	assert.Equal(t, "", cfg.ProjectID)
	assert.Equal(t, "test-image:latest", cfg.MachineAgentImage)
}

func TestLoad_WithEnvVars(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("REGION", "europe-west1")
	t.Setenv("GCP_PROJECT", "my-project")
	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, "europe-west1", cfg.Region)
	assert.Equal(t, "my-project", cfg.ProjectID)
	assert.Equal(t, "test-image:latest", cfg.MachineAgentImage)
}

func TestLoad_PartialEnvVars(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("REGION")
	t.Setenv("GCP_PROJECT", "proj-123")
	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, DefaultEnvironment, cfg.Environment)
	assert.Equal(t, DefaultRegion, cfg.Region)
	assert.Equal(t, "proj-123", cfg.ProjectID)
	assert.Equal(t, "test-image:latest", cfg.MachineAgentImage)
}

func TestLoad_DaprStateStoreNameDefault(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")
	os.Unsetenv("DAPR_STATE_STORE_NAME")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, "statestore", cfg.DaprStateStoreName)
}

func TestLoad_DaprStateStoreNameCustom(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")
	t.Setenv("DAPR_STATE_STORE_NAME", "custom-store")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, "custom-store", cfg.DaprStateStoreName)
}

func TestLoadWithMetadata_FailsOutsideGCE(t *testing.T) {
	// Outside GCE, metadata.ProjectID() should fail
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")

	_, err := LoadWithMetadata()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project ID from metadata")
}
