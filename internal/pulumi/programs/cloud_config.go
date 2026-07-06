package programs

import (
	_ "embed"
	"strings"
)

//go:embed server_cloud_config.yml
var cloudConfigTemplate string

type TemplateConfig struct {
	Region            string
	GCPProject        string
	Environment       string
	InstanceName      string
	BackupBucket      string
	MachineAgentImage string
	MinecraftVersion  string
	RCONPassword      string
	ControllerURL     string
	AgentToken        string
}

func RenderCloudConfig(config *TemplateConfig) (string, error) {
	replacements := map[string]string{
		"${region}":            config.Region,
		"${gcpProject}":        config.GCPProject,
		"${environment}":       config.Environment,
		"${instanceName}":      config.InstanceName,
		"${backupBucket}":      config.BackupBucket,
		"${machineAgentImage}": config.MachineAgentImage,
		"${minecraftVersion}":  config.MinecraftVersion,
		"${rconPassword}":      config.RCONPassword,
		"${controllerUrl}":     config.ControllerURL,
		"${agentToken}":        config.AgentToken,
	}

	result := cloudConfigTemplate
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	return result, nil
}
