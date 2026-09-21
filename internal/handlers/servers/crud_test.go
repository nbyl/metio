package servers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, int(tt.wantUpdate), classifyUpdate(tt.req, nil))
		})
	}
}
