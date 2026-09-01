package programs

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
)

//go:embed server_cloud_config.yml
var cloudConfigTemplate string

type TemplateConfig struct {
	Region               string
	GCPProject           string
	Environment          string
	InstanceName         string
	BackupBucket         string
	ServerID             string
	BackupRetentionDays  int
	ResticPassword       string
	MachineAgentImage    string
	BackupImage          string
	BackupInterval       string
	PruneResticRetention string
	BackupServiceEnable  string
	MinecraftVersion     string
	RCONPassword         string
	ControllerURL        string
	AgentToken           string

	// RestoreSnapshotID is the restic snapshot to restore before Minecraft
	// starts on first boot. When set, a one-off restore step is added to the
	// cloud-config runcmd section.
	RestoreSnapshotID string

	// RestoreSourcePrefix is the restic repository prefix inside the backup
	// bucket that contains the snapshot to restore (e.g.
	// "servers/{old-server-id}/restic"). It differs from the new server's own
	// prefix so the restore reads from the source backup's repository.
	RestoreSourcePrefix string
}

func imageRegistryHost(image string, fallbackRegion string) string {
	host, _, _ := strings.Cut(image, "/")
	if strings.Contains(host, ".") {
		return host
	}
	return fallbackRegion + "-docker.pkg.dev"
}

// normalizeBackupPrefix strips a trailing slash from a restic repository
// prefix. Stored prefixes end in "/" (the backup hook stamps
// "servers/<id>/restic/"), so normalizing keeps the IAM condition and the
// restore repository URL from rendering a doubled slash. The wire format is
// unchanged.
func normalizeBackupPrefix(prefix string) string {
	return strings.TrimSuffix(prefix, "/")
}

func RenderCloudConfig(config *TemplateConfig) (string, error) {
	imageHost := imageRegistryHost(config.MachineAgentImage, config.Region)

	// Normalize defaults so the rendered unit file always has valid values,
	// even when the caller only sets a subset of the template fields.
	if config.BackupRetentionDays == 0 {
		config.BackupRetentionDays = 90
	}
	if config.BackupImage == "" {
		config.BackupImage = "ghcr.io/itzg/mc-backup:latest"
	}
	if config.BackupInterval == "" {
		config.BackupInterval = "1h"
	}
	if config.PruneResticRetention == "" {
		config.PruneResticRetention = fmt.Sprintf("--keep-within %dd", config.BackupRetentionDays)
	}

	// Build the optional first-boot restore step and the guarded service start
	// that follows it. When a snapshot ID and source prefix are provided, a
	// one-off container restores the snapshot into the data directory before
	// any services start. runcmd runs as root, which unlike the systemd units
	// has no registry credential helper configured, so the step configures
	// docker-credential-gcr itself and retries the pull like the units do. On
	// failure a marker file is created and the start command skips Minecraft,
	// so a failed restore can never masquerade as a freshly generated empty
	// world. The marker is removed before attempting, so a later successful run
	// (reboot or re-provision) recovers automatically.
	restoreCommand := ""
	startCommand := fmt.Sprintf("  - systemctl start minecraft %smetio-machine-agent", config.BackupServiceEnable)
	if config.RestoreSnapshotID != "" && config.RestoreSourcePrefix != "" {
		sourcePrefix := normalizeBackupPrefix(config.RestoreSourcePrefix)
		restoreCommand = fmt.Sprintf(
			"  - /bin/bash -c 'rm -f /mnt/disks/minecraft/.metio-restore-failed; docker-credential-gcr configure-docker --registries %s; for i in 1 2 3; do docker pull %s && docker run --rm --name metio-restore --network host -v /mnt/disks/minecraft/data:/data -e RESTIC_REPOSITORY=gs:%s:/%s -e RESTIC_PASSWORD=%s %s restic restore %s:/data --target /data && exit 0; sleep 10; done; touch /mnt/disks/minecraft/.metio-restore-failed; exit 1'",
			imageHost,
			config.BackupImage, config.BackupBucket, sourcePrefix, config.ResticPassword,
			config.BackupImage, config.RestoreSnapshotID,
		)
		startCommand = fmt.Sprintf(
			"  - /bin/bash -c 'if [ -f /mnt/disks/minecraft/.metio-restore-failed ]; then echo \"Metio restore failed; Minecraft left stopped, only the machine agent started.\" >&2; systemctl start metio-machine-agent; exit 1; fi; systemctl start minecraft %smetio-machine-agent'",
			config.BackupServiceEnable,
		)
	}

	replacements := map[string]string{
		"${restoreCommand}":       restoreCommand,
		"${startCommand}":         startCommand,
		"${region}":               config.Region,
		"${gcpProject}":           config.GCPProject,
		"${environment}":          config.Environment,
		"${instanceName}":         config.InstanceName,
		"${backupBucket}":         config.BackupBucket,
		"${serverId}":             config.ServerID,
		"${backupRetentionDays}":  strconv.Itoa(config.BackupRetentionDays),
		"${resticPassword}":       config.ResticPassword,
		"${machineAgentImage}":    config.MachineAgentImage,
		"${backupImage}":          config.BackupImage,
		"${backupInterval}":       config.BackupInterval,
		"${pruneResticRetention}": config.PruneResticRetention,
		"${backupServiceEnable}":  config.BackupServiceEnable,
		"${minecraftVersion}":     config.MinecraftVersion,
		"${rconPassword}":         config.RCONPassword,
		"${controllerUrl}":        config.ControllerURL,
		"${agentToken}":           config.AgentToken,
		"${imageRegistryHost}":    imageHost,
	}

	result := cloudConfigTemplate
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	return result, nil
}
