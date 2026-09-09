package db

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinecraftVersionsFallbackSurfaces26x(t *testing.T) {
	// The offline fallback (used only if Mojang is unreachable and nothing has
	// been cached) must still offer the current biggest release line, so a
	// Mojang outage can never mask 26.x from GET /api/options.
	for _, v := range MinecraftVersions {
		if strings.HasPrefix(v, "26.") {
			return
		}
	}
	assert.Fail(t, "MinecraftVersions fallback does not include any 26.x release")
}