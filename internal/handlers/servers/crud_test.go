package servers

import (
	"testing"

	"github.com/nbyl/metio/internal/db"
	"github.com/stretchr/testify/assert"
)

func modpackConfigPtr(mc db.ModpackConfig) *db.ModpackConfig { return &mc }

func modpackConfigPPtr(mc db.ModpackConfig) **db.ModpackConfig {
	p := &mc
	return &p
}

func TestClassifyUpdate(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name       string
		req        UpdateServerRequest
		wantUpdate UpdateType
	}{
		{
			name:       "no changes is in-place",
			req:        UpdateServerRequest{},
			wantUpdate: UpdateTypeInPlace,
		},
		{
			name: "machine type change is a resize",
			req: UpdateServerRequest{
				MachineType: strPtr("n2-standard-4"),
			},
			wantUpdate: UpdateTypeResize,
		},
		{
			name: "version change is a recreate",
			req: UpdateServerRequest{
				MinecraftVersion: strPtr("1.21.1"),
			},
			wantUpdate: UpdateTypeRecreate,
		},
		{
			name: "version change wins over machine type",
			req: UpdateServerRequest{
				MachineType:      strPtr("n2-standard-4"),
				MinecraftVersion: strPtr("1.21.1"),
			},
			wantUpdate: UpdateTypeRecreate,
		},
		{
			name: "setting a pack is a recreate",
			req: UpdateServerRequest{
				Modpack: modpackConfigPPtr(db.ModpackConfig{Platform: "modrinth", ProjectID: "abC123"}),
			},
			wantUpdate: UpdateTypeRecreate,
		},
		{
			name: "removing a pack is a recreate",
			req: UpdateServerRequest{
				Modpack: modpackConfigPPtr(db.ModpackConfig{}),
			},
			wantUpdate: UpdateTypeRecreate,
		},
		{
			name: "pack change wins over version",
			req: UpdateServerRequest{
				MinecraftVersion: strPtr("1.21.1"),
				Modpack:          modpackConfigPPtr(db.ModpackConfig{Platform: "modrinth", ProjectID: "abC123"}),
			},
			wantUpdate: UpdateTypeRecreate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, int(tt.wantUpdate), classifyUpdate(tt.req, nil))
		})
	}
}
