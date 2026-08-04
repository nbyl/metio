package programs

import (
	"testing"

	"github.com/stretchr/testify/assert"
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