package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateServerName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "myserver", false},
		{"valid with hyphens", "my-server-123", false},
		{"valid single char", "a", true},
		{"valid two chars", "ab", true},
		{"valid three chars", "abc", false},
		{"too long 64 chars", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"valid max length 63 chars", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaAAAAAAAAAAA", false},
		{"starts with hyphen", "-server", true},
		{"ends with hyphen", "server-", true},
		{"contains underscore", "my_server", true},
		{"contains space", "my server", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServerName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRegion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid europe-west3", "europe-west3", false},
		{"valid us-central1", "us-central1", false},
		{"valid asia-east1", "asia-east1", false},
		{"empty", "", true},
		{"invalid region", "invalid-region", true},
		{"invalid zone used as region", "europe-west3-a", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegion(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateZone(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid zone", "europe-west3-a", false},
		{"valid zone b", "us-central1-b", false},
		{"empty", "", true},
		{"region only", "europe-west3", true},
		{"invalid zone", "europe-west3-x", true},
		{"invalid zone for region", "us-east1-a", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateZone(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMachineType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid e2-small", "e2-small", false},
		{"valid e2-medium", "e2-medium", false},
		{"valid n2-standard-2", "n2-standard-2", false},
		{"valid n2-highmem-4", "n2-highmem-4", false},
		{"empty", "", true},
		{"invalid type", "invalid-type", true},
		{"invalid prefix", "x2-small", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMachineType(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDiskSize(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		wantErr bool
	}{
		{"minimum valid", 10, false},
		{"maximum valid", 1000, false},
		{"mid range", 100, false},
		{"below minimum", 9, true},
		{"above maximum", 1001, true},
		{"zero", 0, true},
		{"negative", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDiskSize(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateServerConfig(t *testing.T) {
	validConfig := &ServerConfig{
		Name:             "test-server",
		Region:           "europe-west3",
		Zone:             "europe-west3-a",
		MachineType:      "e2-small",
		MinecraftVersion: "1.21.1",
		DiskSizeGB:       50,
	}

	t.Run("nil config", func(t *testing.T) {
		err := ValidateServerConfig(nil)
		assert.Error(t, err)
	})

	t.Run("valid config", func(t *testing.T) {
		err := ValidateServerConfig(validConfig)
		assert.NoError(t, err)
	})

	t.Run("config with valid shutdown schedule", func(t *testing.T) {
		config := *validConfig
		config.ShutdownSchedule = &ShutdownSchedule{
			Enabled:  true,
			Time:     "21:00",
			Timezone: "Europe/Berlin",
		}
		err := ValidateServerConfig(&config)
		assert.NoError(t, err)
	})

	t.Run("config with disabled shutdown schedule", func(t *testing.T) {
		config := *validConfig
		config.ShutdownSchedule = &ShutdownSchedule{
			Enabled: false,
		}
		err := ValidateServerConfig(&config)
		assert.NoError(t, err)
	})

	t.Run("config with invalid shutdown schedule time", func(t *testing.T) {
		config := *validConfig
		config.ShutdownSchedule = &ShutdownSchedule{
			Enabled:  true,
			Time:     "25:00",
			Timezone: "Europe/Berlin",
		}
		err := ValidateServerConfig(&config)
		assert.Error(t, err)
	})

	t.Run("config with invalid shutdown schedule timezone", func(t *testing.T) {
		config := *validConfig
		config.ShutdownSchedule = &ShutdownSchedule{
			Enabled:  true,
			Time:     "21:00",
			Timezone: "Invalid/Timezone",
		}
		err := ValidateServerConfig(&config)
		assert.Error(t, err)
	})
}

func TestShutdownScheduleIsValid(t *testing.T) {
	t.Run("nil schedule", func(t *testing.T) {
		var schedule *ShutdownSchedule
		assert.True(t, schedule.IsValid())
	})

	t.Run("disabled schedule", func(t *testing.T) {
		schedule := &ShutdownSchedule{Enabled: false}
		assert.True(t, schedule.IsValid())
	})

	t.Run("enabled with empty time", func(t *testing.T) {
		schedule := &ShutdownSchedule{Enabled: true, Timezone: "Europe/Berlin"}
		assert.False(t, schedule.IsValid())
	})

	t.Run("enabled with empty timezone", func(t *testing.T) {
		schedule := &ShutdownSchedule{Enabled: true, Time: "21:00"}
		assert.False(t, schedule.IsValid())
	})

	t.Run("enabled with valid time and timezone", func(t *testing.T) {
		schedule := &ShutdownSchedule{Enabled: true, Time: "21:00", Timezone: "Europe/Berlin"}
		assert.True(t, schedule.IsValid())
	})
}

func TestIsValidMinecraftVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid 1.21.1", "1.21.1", true},
		{"valid 1.20.4", "1.20.4", true},
		{"valid 1.7.10", "1.7.10", true},
		{"invalid version", "2.0.0", false},
		{"invalid format", "latest", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidMinecraftVersion(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMachineTypes(t *testing.T) {
	t.Run("e2-small exists", func(t *testing.T) {
		spec, ok := MachineTypes["e2-small"]
		assert.True(t, ok)
		assert.Equal(t, 2, spec.VCPUs)
		assert.Equal(t, 2, spec.MemoryGB)
		assert.Greater(t, spec.MonthlyCost, 0.0)
	})

	t.Run("n2-highmem-16 exists", func(t *testing.T) {
		spec, ok := MachineTypes["n2-highmem-16"]
		assert.True(t, ok)
		assert.Equal(t, 16, spec.VCPUs)
		assert.Equal(t, 128, spec.MemoryGB)
	})

	t.Run("invalid type", func(t *testing.T) {
		_, ok := MachineTypes["invalid"]
		assert.False(t, ok)
	})
}
