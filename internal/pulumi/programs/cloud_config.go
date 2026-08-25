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

	// Build the optional restore command. When a snapshot ID is provided, a
	// one-off docker run restores the snapshot into the data directory before
	// any services start.
	restoreCommand := ""
	if config.RestoreSnapshotID != "" && config.RestoreSourcePrefix != "" {
		restoreCommand = fmt.Sprintf(
			"  - docker run --rm --name metio-restore --network host -v /mnt/disks/minecraft/data:/data -e RESTIC_REPOSITORY=gs:%s:/%s -e RESTIC_PASSWORD=%s %s restic restore %s --target /data",
			config.BackupBucket, config.RestoreSourcePrefix, config.ResticPassword,
			config.BackupImage, config.RestoreSnapshotID,
		)
	}

	replacements := map[string]string{
		"${restoreCommand}":       restoreCommand,
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
