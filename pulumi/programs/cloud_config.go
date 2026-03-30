package programs

import (
	"os"
	"path/filepath"
	"strings"
)

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
	// Try to read from the shared templates directory
	paths := []string{
		filepath.Join("..", "templates", "server_cloud_config.tftpl"),
		filepath.Join("templates", "server_cloud_config.tftpl"),
		"server_cloud_config.tftpl",
	}

	var templateContent string
	for _, p := range paths {
		content, err := os.ReadFile(p)
		if err == nil {
			templateContent = string(content)
			break
		}
	}

	if templateContent == "" {
		// Fallback: return empty string - should not happen in production
		return "", nil
	}

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

	result := templateContent
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	return result, nil
}
