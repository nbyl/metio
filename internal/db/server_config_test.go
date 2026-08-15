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
		{"too long 25 chars", "abcdefghijklmnopqrstuvwxy", true},
		{"valid max length 24 chars", "abcdefghijklmnopqrstuvwx", false},
		{"starts with hyphen", "-server", true},
		{"ends with hyphen", "server-", true},
		{"contains underscore", "my_server", true},
		{"contains space", "my server", true},
		{"contains uppercase", "MyServer", true},
		{"starts with digit", "1server", true},
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
		{"uppercase", "US-Central1", true},
		{"spaces", "us central1", true},
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
		{"uppercase", "Europe-West3-A", true},
		{"spaces", "europe west3 a", true},
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
		{"valid any family", "x2-mega", false},
		{"empty", "", true},
		{"uppercase", "E2-small", true},
		{"space", "e2 small", true},
		{"leading dash", "-e2-small", true},
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

func TestBackupConfigIsValid(t *testing.T) {
	testCases := []struct {
		name    string
		config  *BackupConfig
		wantErr bool
	}{
		{name: "nil uses deployment defaults", config: nil, wantErr: false},
		{name: "default enabled", config: &BackupConfig{Enabled: true}, wantErr: false},
		{name: "disabled", config: &BackupConfig{Enabled: false}, wantErr: false},
		{name: "interval in hours", config: &BackupConfig{Enabled: true, BackupIntervalHours: 6}, wantErr: false},
		{name: "retention policy", config: &BackupConfig{Enabled: true, Keep: 7, KeepUnit: "daily"}, wantErr: false},
		{name: "keep without unit", config: &BackupConfig{Enabled: true, Keep: 7}, wantErr: true},
		{name: "unit without keep", config: &BackupConfig{Enabled: true, KeepUnit: "daily"}, wantErr: false},
		{name: "unsupported unit", config: &BackupConfig{Enabled: true, Keep: 7, KeepUnit: "fortnightly"}, wantErr: true},
		{name: "negative interval", config: &BackupConfig{Enabled: true, BackupIntervalHours: -1}, wantErr: true},
		{name: "negative retention", config: &BackupConfig{Enabled: true, Keep: -1, KeepUnit: "daily"}, wantErr: true},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.IsValid()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateServerConfig_Backup(t *testing.T) {
	validBase := &ServerConfig{
		Name:             "test-server",
		Region:           "europe-west3",
		Zone:             "europe-west3-a",
		MachineType:      "e2-small",
		MinecraftVersion: "1.21.1",
		DiskSizeGB:       50,
	}

	t.Run("valid backup config", func(t *testing.T) {
		config := *validBase
		config.Backup = &BackupConfig{Enabled: true, BackupIntervalHours: 6, Keep: 7, KeepUnit: "daily"}
		assert.NoError(t, ValidateServerConfig(&config))
	})

	t.Run("backup disabled", func(t *testing.T) {
		config := *validBase
		config.Backup = &BackupConfig{Enabled: false}
		assert.NoError(t, ValidateServerConfig(&config))
	})

	t.Run("invalid backup interval", func(t *testing.T) {
		config := *validBase
		config.Backup = &BackupConfig{Enabled: true, BackupIntervalHours: -1}
		assert.Error(t, ValidateServerConfig(&config))
	})
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
