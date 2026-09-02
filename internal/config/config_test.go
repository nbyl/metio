package config

import (
	"os"
	"testing"
	"time"

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

func TestLoad_BackupImage(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")

	t.Run("empty falls back to empty (program defaults to upstream mc-backup)", func(t *testing.T) {
		os.Unsetenv("BACKUP_IMAGE")
		cfg, err := Load()
		assert.NoError(t, err)
		assert.Equal(t, "", cfg.BackupImage)
	})

	t.Run("set is read", func(t *testing.T) {
		t.Setenv("BACKUP_IMAGE", "europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:1.0.0")
		cfg, err := Load()
		assert.NoError(t, err)
		assert.Equal(t, "europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:1.0.0", cfg.BackupImage)
	})
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

func TestLoad_BackupDeletedServerRetentionDaysDefault(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")
	os.Unsetenv("BACKUP_DELETED_SERVER_RETENTION_DAYS")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, DefaultBackupDeletedServerRetentionDays, cfg.BackupDeletedServerRetentionDays)
}

func TestLoad_BackupDeletedServerRetentionDaysCustom(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")
	t.Setenv("BACKUP_DELETED_SERVER_RETENTION_DAYS", "60")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, 60, cfg.BackupDeletedServerRetentionDays)
}

func TestLoad_BackupDeletedServerRetentionDaysInvalid(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")

	for _, raw := range []string{"0", "-5", "abc"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("BACKUP_DELETED_SERVER_RETENTION_DAYS", raw)

			_, err := Load()

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "BACKUP_DELETED_SERVER_RETENTION_DAYS")
		})
	}
}

func TestLoad_BackupResticPassword(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")
	t.Setenv("BACKUP_RESTIC_PASSWORD", "super-secret-password")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, "super-secret-password", cfg.BackupResticPassword)
}

func TestLoad_SaveAckTimeoutDefault(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")
	os.Unsetenv("SAVE_ACK_TIMEOUT")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, DefaultSaveAckTimeout, cfg.SaveAckTimeout)
}

func TestLoad_SaveAckTimeoutCustom(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")
	t.Setenv("SAVE_ACK_TIMEOUT", "5m")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.Equal(t, 5*time.Minute, cfg.SaveAckTimeout)
}

func TestLoad_SaveAckTimeoutInvalid(t *testing.T) {
	viper.Reset()
	viper.AutomaticEnv()

	t.Setenv("MACHINE_AGENT_IMAGE", "test-image:latest")

	for _, raw := range []string{"0", "-1s", "abc"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("SAVE_ACK_TIMEOUT", raw)

			_, err := Load()

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "SAVE_ACK_TIMEOUT")
		})
	}
}
