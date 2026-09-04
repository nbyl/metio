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

	// Build the optional first-boot restore: a one-shot systemd unit that
	// restores the snapshot into the data directory before any services start.
	// Running as the metio user (as the service units do) gives a writable HOME
	// so docker-credential-gcr can write its config; ordering is guaranteed by
	// Minecraft's After=/Requires=.
	//
	// The unit is written to the disk on first boot. It runs exactly once: on an
	// already-restored machine the run-once sentinel keyed to the snapshot ID
	// makes ConditionPathExists skip it, and runcmd/cloud-init re-run on COS on
	// every boot (users-groups is once, scripts-user/runcmd are "always"). If
	// the restore fails the unit is skipped-but-satisfied, so Minecraft never
	// starts against an empty world and the machine agent still reports.
	afterRestore := ""
	requiresRestore := ""
	restoreWriteFiles := ""
	if config.RestoreSnapshotID != "" && config.RestoreSourcePrefix != "" {
		// The restore dependency lines are injected inline into the
		// minecraft.service unit, at the same block indentation, so an empty
		// (non-restore) render leaves no dangling blank line inside the YAML
		// literal block.
		afterRestore = "\n      After=metio-restore.service"
		requiresRestore = "\n      Requires=metio-restore.service"
		sourcePrefix := normalizeBackupPrefix(config.RestoreSourcePrefix)
		// Values interpolated into a systemd unit are escaped: '%' becomes '%%'
		// so %n-style specifiers are not mangled in the rendered content.
		restoreWriteFiles = fmt.Sprintf(`  - path: /etc/systemd/system/metio-restore.service
    permissions: '0644'
    content: |
      [Unit]
      Description=Metio One-Shot World Restore
      After=docker.service
      Requires=docker.service
      Before=minecraft.service
      ConditionPathExists=!/mnt/disks/minecraft/.metio-restore-%s.done

      [Service]
      Type=oneshot
      RemainAfterExit=yes
      User=metio
      WorkingDirectory=/home/metio
      ExecStartPre=/usr/bin/docker-credential-gcr configure-docker --registries %s
      ExecStartPre=/bin/bash -c 'for i in 1 2 3; do docker pull %s && exit 0; sleep 10; done; exit 1'
      ExecStart=/usr/bin/docker run --rm --name metio-restore --network host \
        -e RESTIC_REPOSITORY=gs:%s:/%s \
        -e RESTIC_PASSWORD=%s \
        -v /mnt/disks/minecraft/data:/data \
        %s restic restore %s:/data --target /data
      ExecStartPost=+/bin/touch /mnt/disks/minecraft/.metio-restore-%s.done
`, config.RestoreSnapshotID, imageHost, config.BackupImage,
			config.BackupBucket, sourcePrefix, config.ResticPassword,
			config.BackupImage, config.RestoreSnapshotID, config.RestoreSnapshotID)
	}
	startCommand := fmt.Sprintf("  - systemctl start minecraft %smetio-machine-agent", config.BackupServiceEnable)

	replacements := map[string]string{
		"${restoreWriteFiles}":    restoreWriteFiles,
		"${afterRestore}":         afterRestore,
		"${requiresRestore}":      requiresRestore,
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
