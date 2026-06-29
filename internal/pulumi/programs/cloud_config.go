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
	}

	result := cloudConfigTemplate
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	return result, nil
}
