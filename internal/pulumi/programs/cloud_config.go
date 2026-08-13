package programs

import (
	_ "embed"
	"strconv"
	"strings"
)

//go:embed server_cloud_config.yml
var cloudConfigTemplate string

type TemplateConfig struct {
	Region              string
	GCPProject          string
	Environment         string
	InstanceName        string
	BackupBucket        string
	ServerID            string
	BackupRetentionDays int
	ResticPassword      string
	MachineAgentImage   string
	MinecraftVersion    string
	RCONPassword        string
	ControllerURL       string
	AgentToken          string
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

	replacements := map[string]string{
		"${region}":              config.Region,
		"${gcpProject}":          config.GCPProject,
		"${environment}":         config.Environment,
		"${instanceName}":        config.InstanceName,
		"${backupBucket}":        config.BackupBucket,
		"${serverId}":            config.ServerID,
		"${backupRetentionDays}": strconv.Itoa(config.BackupRetentionDays),
		"${resticPassword}":      config.ResticPassword,
		"${machineAgentImage}":   config.MachineAgentImage,
		"${minecraftVersion}":    config.MinecraftVersion,
		"${rconPassword}":        config.RCONPassword,
		"${controllerUrl}":       config.ControllerURL,
		"${agentToken}":          config.AgentToken,
		"${imageRegistryHost}":   imageHost,
	}

	result := cloudConfigTemplate
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	return result, nil
}
