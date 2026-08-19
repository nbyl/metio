package tasks

import (
	"testing"

	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/pulumi/programs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildProgramConfig_PropagatesBackupImage(t *testing.T) {
	cfg := config.Config{
		Environment:          "development2",
		ProjectID:            "minecraftbyl",
		MachineAgentImage:    "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:58edad6",
		BackupImage:          "europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:58edad6",
		BackupResticPassword: "restic-secret",
	}
	sc := &db.ServerConfig{
		ID:               "server-1",
		Name:             "test-server",
		Region:           "europe-west3",
		Zone:             "europe-west3-b",
		MachineType:      "e2-small",
		MinecraftVersion: "1.20.1",
		DiskSizeGB:       10,
		ExistingAddress:  "test-server-addr",
		InfraVersion:     programs.CurrentInfraVersion,
	}

	programConfig := buildProgramConfig(sc, cfg, "token")

	require.NotNil(t, programConfig)
	assert.Equal(t, cfg.BackupImage, programConfig.BackupImage)
	assert.Equal(t, cfg.MachineAgentImage, programConfig.MachineAgentImage)
	assert.Equal(t, cfg.ProjectID, programConfig.GCPProject)
	assert.Equal(t, sc.Name, programConfig.Name)
	assert.Nil(t, programConfig.Backup)
}

func TestBuildProgramConfig_PropagatesPerServerBackupOverride(t *testing.T) {
	cfg := config.Config{
		Environment: "development2",
		ProjectID:   "minecraftbyl",
		BackupImage: "europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:58edad6",
	}
	sc := &db.ServerConfig{
		ID:               "server-1",
		Name:             "test-server",
		Region:           "europe-west3",
		Zone:             "europe-west3-b",
		MinecraftVersion: "1.20.1",
		Backup: &db.BackupConfig{
			Enabled:             true,
			BackupIntervalHours: 3,
			Keep:                14,
			KeepUnit:            "daily",
		},
	}

	programConfig := buildProgramConfig(sc, cfg, "token")

	require.NotNil(t, programConfig)
	require.NotNil(t, programConfig.Backup)
	assert.Equal(t, cfg.BackupImage, programConfig.BackupImage)
	assert.True(t, programConfig.Backup.Enabled)
	assert.Equal(t, 3, programConfig.Backup.BackupIntervalHours)
	assert.Equal(t, 14, programConfig.Backup.Keep)
	assert.Equal(t, "daily", programConfig.Backup.KeepUnit)
}
