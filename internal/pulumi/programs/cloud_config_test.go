package programs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestImageRegistryHost(t *testing.T) {
	tests := []struct {
		name           string
		image          string
		fallbackRegion string
		want           string
	}{
		{
			name:           "fully qualified AR image",
			image:          "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
			fallbackRegion: "europe-west2",
			want:           "europe-west3-docker.pkg.dev",
		},
		{
			name:           "multi-region AR host",
			image:          "europe-docker.pkg.dev/metio-distribution/metio/machine-agent:1.5.0",
			fallbackRegion: "europe-west2",
			want:           "europe-docker.pkg.dev",
		},
		{
			name:           "bare image with no host (guard fallback)",
			image:          "machine-agent:latest",
			fallbackRegion: "europe-west2",
			want:           "europe-west2-docker.pkg.dev",
		},
		{
			name:           "short bare image fallback",
			image:          "alpine",
			fallbackRegion: "europe-west3",
			want:           "europe-west3-docker.pkg.dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageRegistryHost(tt.image, tt.fallbackRegion)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderCloudConfig_ImageRegistryHost(t *testing.T) {
	cfg := &TemplateConfig{
		Region:            "europe-west2",
		MachineAgentImage: "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)
	assert.Contains(t, result, "--registries europe-west3-docker.pkg.dev")
}

func TestRenderCloudConfig_ImageRegistryHostFallback(t *testing.T) {
	cfg := &TemplateConfig{
		Region:            "europe-west2",
		MachineAgentImage: "machine-agent:latest",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)
	assert.Contains(t, result, "--registries europe-west2-docker.pkg.dev")
}

func TestRenderCloudConfig_CentralBackupSettings(t *testing.T) {
	cfg := &TemplateConfig{
		Region:              "europe-west3",
		MachineAgentImage:   "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
		BackupBucket:        "my-project-development-backups",
		ServerID:            "0dcbaca4-2a26-489c-b4a3-d2fad8bb6483",
		BackupRetentionDays: 90,
		ResticPassword:      "deployment-wide-restic-password",
		RCONPassword:        "rcon-password",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)

	repo := "gs:my-project-development-backups:/servers/0dcbaca4-2a26-489c-b4a3-d2fad8bb6483/restic"
	assert.Contains(t, result, "RESTIC_REPOSITORY="+repo)
	assert.Contains(t, result, "PRUNE_RESTIC_RETENTION=\"--keep-within 90d\"")
	assert.Contains(t, result, "RESTIC_PASSWORD=deployment-wide-restic-password")
	// The backup hook needs the server ID to stamp manifests for the
	// machine-agent's backup reporting (ADR-0004).
	assert.Contains(t, result, "METIO_SERVER_ID=0dcbaca4-2a26-489c-b4a3-d2fad8bb6483")
	assert.Contains(t, result, "RCON_PASSWORD="+cfg.RCONPassword)
	assert.NotContains(t, result, "RESTIC_REPOSITORY=gs:"+cfg.BackupBucket+":/ \\")
	assert.NotContains(t, result, "keep-within 3m")
}

func TestRenderCloudConfig_DiskLayoutAndManifestMount(t *testing.T) {
	cfg := &TemplateConfig{
		Region:              "europe-west3",
		MachineAgentImage:   "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
		BackupBucket:        "my-project-development-backups",
		ServerID:            "0dcbaca4-2a26-489c-b4a3-d2fad8bb6483",
		BackupRetentionDays: 90,
		ResticPassword:      "deployment-wide-restic-password",
		RCONPassword:        "rcon-password",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)

	// The data disk now mounts at /mnt/disks/minecraft with world data in a
	// data/ subdirectory (InfraVersion 4 layout).
	assert.Contains(t, result, "[minecraft_data, /mnt/disks/minecraft, ext4, \"defaults\", \"0\", \"2\"]")
	assert.NotContains(t, result, "[minecraft_data, /mnt/disks/minecraft/data, ext4")

	// The manifest queue is a parallel directory on the same disk, shared by
	// the backup and machine-agent containers as /manifests.
	assert.Contains(t, result, "-v /mnt/disks/minecraft/backup-manifests:/manifests")
	assert.Contains(t, result, "mkdir -p /mnt/disks/minecraft/data /mnt/disks/minecraft/backup-manifests")
	assert.Contains(t, result, "chmod 0777 /mnt/disks/minecraft/backup-manifests")

	// Both backup and agent containers still mount the world data directory.
	assert.Contains(t, result, "-v /mnt/disks/minecraft/data:/data")
}

func TestRenderCloudConfig_BackupOverrides(t *testing.T) {
	cfg := &TemplateConfig{
		Region:               "europe-west3",
		MachineAgentImage:    "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
		BackupImage:          "europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:1.0.0",
		BackupInterval:       "6h",
		PruneResticRetention: "--keep-last 3 --keep-daily 10",
		BackupServiceEnable:  "minecraft-backup ",
		RCONPassword:         "rcon-password",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)

	assert.Contains(t, result, "docker pull europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:1.0.0")
	assert.Contains(t, result, "BACKUP_INTERVAL=\"6h\"")
	assert.Contains(t, result, "PRUNE_RESTIC_RETENTION=\"--keep-last 3 --keep-daily 10\"")
	assert.Contains(t, result, "systemctl enable minecraft minecraft-backup metio-machine-agent")
	assert.Contains(t, result, "systemctl start minecraft minecraft-backup metio-machine-agent")
}

func TestRenderCloudConfig_BackupDisabled(t *testing.T) {
	cfg := &TemplateConfig{
		Region:               "europe-west3",
		MachineAgentImage:    "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
		BackupImage:          "ghcr.io/itzg/mc-backup:latest",
		BackupInterval:       "1h",
		PruneResticRetention: "--keep-within 90d",
		BackupServiceEnable:  "",
		RCONPassword:         "rcon-password",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)

	assert.Contains(t, result, "systemctl enable minecraft metio-machine-agent")
	assert.Contains(t, result, "systemctl start minecraft metio-machine-agent")
	assert.NotContains(t, result, "minecraft-backup metio-machine-agent")
}

func TestRenderCloudConfig_RestoreSnapshotOmitted(t *testing.T) {
	cfg := &TemplateConfig{
		Region:              "europe-west3",
		MachineAgentImage:   "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
		BackupBucket:        "my-project-development-backups",
		ServerID:            "new-server-id",
		BackupRetentionDays: 90,
		ResticPassword:      "restic-pw",
		RCONPassword:        "rcon-pw",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)

	assert.NotContains(t, result, "metio-restore")
	assert.NotContains(t, result, "restic restore")
}

func TestRenderCloudConfig_RestoreSnapshotPresent(t *testing.T) {
	cfg := &TemplateConfig{
		Region:              "europe-west3",
		MachineAgentImage:   "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
		BackupBucket:        "my-project-development-backups",
		ServerID:            "new-server-id",
		BackupRetentionDays: 90,
		ResticPassword:      "restic-pw",
		RCONPassword:        "rcon-pw",
		BackupImage:         "ghcr.io/itzg/mc-backup:latest",
		BackupServiceEnable: "minecraft-backup ",
		RestoreSnapshotID:   "abc123-snapshot",
		RestoreSourcePrefix: "servers/old-server-id/restic",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)

	assert.Contains(t, result, "docker run --rm --name metio-restore")
	assert.Contains(t, result, "docker-credential-gcr configure-docker --registries europe-west3-docker.pkg.dev")
	assert.Contains(t, result, "RESTIC_REPOSITORY=gs:my-project-development-backups:/servers/old-server-id/restic")
	assert.Contains(t, result, "RESTIC_PASSWORD=restic-pw")
	assert.Contains(t, result, "restic restore abc123-snapshot:/data --target /data")
	assert.Contains(t, result, "-v /mnt/disks/minecraft/data:/data")
	assert.Contains(t, result, "rm -f /mnt/disks/minecraft/.metio-restore-failed")
	assert.Contains(t, result, "touch /mnt/disks/minecraft/.metio-restore-failed")

	// Restore step appears before the guarded systemctl start.
	restoreIdx := strings.Index(result, "metio-restore")
	startIdx := strings.Index(result, "systemctl start minecraft")
	assert.True(t, restoreIdx < startIdx, "restore must appear before systemctl start")

	// The guarded start skips Minecraft when the restore failed, but starts it
	// normally on success.
	assert.Contains(t, result, "[ -f /mnt/disks/minecraft/.metio-restore-failed ]")
	assert.Contains(t, result, "systemctl start metio-machine-agent")
	assert.Contains(t, result, "systemctl start minecraft minecraft-backup metio-machine-agent")
}

func TestRenderCloudConfig_RestoreSnapshotOnly(t *testing.T) {
	// When only RestoreSnapshotID is set but RestoreSourcePrefix is empty,
	// no restore step should be rendered (both must be present).
	cfg := &TemplateConfig{
		Region:            "europe-west3",
		MachineAgentImage: "machine-agent:latest",
		RestoreSnapshotID: "abc123",
		RCONPassword:      "rcon-pw",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)

	assert.NotContains(t, result, "metio-restore")
}

func TestRenderCloudConfig_RestoreSourcePrefixTrailingSlash(t *testing.T) {
	// Stored repository prefixes end in "/" (the wire format is validated by
	// the backup report handler), so the restore URL and start guard must
	// not render a doubled slash.
	cfg := &TemplateConfig{
		Region:              "europe-west3",
		MachineAgentImage:   "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
		BackupBucket:        "my-project-development-backups",
		ServerID:            "new-server-id",
		BackupRetentionDays: 90,
		ResticPassword:      "restic-pw",
		RCONPassword:        "rcon-pw",
		BackupImage:         "ghcr.io/itzg/mc-backup:latest",
		RestoreSnapshotID:   "abc123-snapshot",
		RestoreSourcePrefix: "servers/old-server-id/restic/",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)

	assert.Contains(t, result, "RESTIC_REPOSITORY=gs:my-project-development-backups:/servers/old-server-id/restic")
	assert.NotContains(t, result, "restic//")
}

func TestRenderCloudConfig_NoRestoreEmitsPlainStart(t *testing.T) {
	// Without a restore, the start command must be byte-identical to the
	// historical form so the normal boot path is untouched.
	cfg := &TemplateConfig{
		Region:              "europe-west3",
		MachineAgentImage:   "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
		BackupServiceEnable: "minecraft-backup ",
		RCONPassword:        "rcon-pw",
	}
	result, err := RenderCloudConfig(cfg)
	assert.NoError(t, err)

	assert.Contains(t, result, "systemctl start minecraft minecraft-backup metio-machine-agent")
	assert.NotContains(t, result, "metio-restore-failed")
}

func TestRenderCloudConfig_YAMLValid(t *testing.T) {
	// The restore and guarded-start entries are plain YAML scalars; a stray
	// indicator would silently corrupt the whole user-data document. Parse the
	// rendered output to guarantee the VM would receive valid cloud-config.
	tests := []*TemplateConfig{
		{
			Region:              "europe-west3",
			MachineAgentImage:   "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
			BackupBucket:        "my-project-development-backups",
			ServerID:            "new-server-id",
			BackupRetentionDays: 90,
			ResticPassword:      "restic-pw",
			RCONPassword:        "rcon-pw",
			BackupImage:         "ghcr.io/itzg/mc-backup:latest",
			BackupServiceEnable: "minecraft-backup ",
			RestoreSnapshotID:   "abc123-snapshot",
			RestoreSourcePrefix: "servers/old-server-id/restic/",
		},
		{
			Region:              "europe-west3",
			MachineAgentImage:   "europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:tag",
			BackupBucket:        "my-project-development-backups",
			ServerID:            "new-server-id",
			BackupRetentionDays: 90,
			ResticPassword:      "restic-pw",
			RCONPassword:        "rcon-pw",
			BackupImage:         "ghcr.io/itzg/mc-backup:latest",
			RestoreSnapshotID:   "abc123-snapshot",
			RestoreSourcePrefix: "servers/old-server-id/restic",
		},
		{
			Region:              "europe-west3",
			MachineAgentImage:   "machine-agent:latest",
			BackupServiceEnable: "minecraft-backup ",
			RCONPassword:        "rcon-pw",
		},
	}
	for _, cfg := range tests {
		result, err := RenderCloudConfig(cfg)
		assert.NoError(t, err)

		var parsed map[string]interface{}
		assert.NoError(t, yaml.Unmarshal([]byte(result), &parsed))
		runcmd, ok := parsed["runcmd"].([]interface{})
		assert.True(t, ok, "runcmd must be a list")
		assert.NotEmpty(t, runcmd)
	}
}
