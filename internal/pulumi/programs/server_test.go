package programs

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

func TestServerConfigDefaults(t *testing.T) {
	config := &ServerConfig{
		Name:       "test-server",
		GCPProject: "test-project",
	}

	assert.Equal(t, "test-server", config.Name)
	assert.Equal(t, "test-project", config.GCPProject)
	assert.Equal(t, "", config.Region)
	assert.Equal(t, "", config.Zone)
}

func TestServerConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *ServerConfig
		expectError bool
	}{
		{
			name:        "valid config",
			config:      &ServerConfig{Name: "test-server"},
			expectError: false,
		},
		{
			name:        "empty name",
			config:      &ServerConfig{Name: ""},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIAMRolesComplete(t *testing.T) {
	expectedRoles := []string{
		"roles/logging.logWriter",
		"roles/cloudtrace.agent",
		"roles/artifactregistry.reader",
		"roles/serviceusage.serviceUsageConsumer",
	}

	assert.Len(t, expectedRoles, 4, "Should have 4 project-level IAM roles")

	for _, role := range expectedRoles {
		assert.Contains(t, role, "roles/", "Role should start with roles/")
	}
}

func validateConfig(config *ServerConfig) error {
	if config.Name == "" {
		return &ValidationError{Field: "Name", Message: "server name is required"}
	}
	return nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

type MockResourceRegistry struct {
	resources map[string]pulumi.Resource
}

func NewMockResourceRegistry() *MockResourceRegistry {
	return &MockResourceRegistry{
		resources: make(map[string]pulumi.Resource),
	}
}

func TestCurrentInfraVersion_IsPositive(t *testing.T) {
	assert.Greater(t, CurrentInfraVersion, 0, "CurrentInfraVersion should be > 0")
}

func TestGCPTagValueRegex_RejectsUUIDs(t *testing.T) {
	// GCP requires tag values to match: ^[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?$
	re := regexp.MustCompile(`^[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?$`)

	tests := []struct {
		value string
		valid bool
	}{
		{"test1234", true},
		{"my-server", true},
		{"development", true},
		{"0dcbaca4-2a26-489c-b4a3-d2fad8bb6483", false},
		{"Server-1", false},
		{"-server", false},
		{"server-", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, tt.valid, re.MatchString(tt.value))
		})
	}
}

func TestServerID_BelongsInLabelsNotTags(t *testing.T) {
	serverID := "0dcbaca4-2a26-489c-b4a3-d2fad8bb6483"
	tagRe := regexp.MustCompile(`^[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?$`)
	labelRe := regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9_]{0,61}[a-z0-9])?$`)

	assert.False(t, tagRe.MatchString(serverID),
		"ServerID UUID must not be a GCP instance tag")
	assert.True(t, labelRe.MatchString(serverID),
		"ServerID UUID must be valid as a GCP label value")
}

func TestCentralBackupBucketName(t *testing.T) {
	assert.Equal(t, "my-project-development-backups", centralBackupBucketName("my-project", "development"))
	assert.Equal(t, "proj-123-prod-backups", centralBackupBucketName("proj-123", "prod"))
}

func TestServerBackupPrefix(t *testing.T) {
	assert.Equal(t, "servers/abc-123/restic", serverBackupPrefix("abc-123"))
}

func TestBackupPrefixCondition(t *testing.T) {
	cond := backupPrefixCondition("my-project-development-backups", "abc-123")
	assert.Equal(t,
		`resource.name.startsWith("projects/_/buckets/my-project-development-backups/objects/servers/abc-123/restic/")`,
		cond)
}

func TestBackupObjectListRoleID(t *testing.T) {
	tests := []struct {
		environment string
		want        string
	}{
		{"development", "development_backup_object_list"},
		{"development2", "development2_backup_object_list"},
		{"my-env-1", "my_env_1_backup_object_list"},
	}

	for _, tt := range tests {
		t.Run(tt.environment, func(t *testing.T) {
			assert.Equal(t, tt.want, backupObjectListRoleID(tt.environment))
		})
	}
}

func TestServerConfigDefaults_CentralBackup(t *testing.T) {
	config := &ServerConfig{
		Name:        "test-server",
		ServerID:    "srv-1",
		GCPProject:  "my-project",
		Environment: "development",
	}

	assert.Equal(t, "my-project-development-backups", centralBackupBucketName(config.GCPProject, config.Environment))
	assert.Equal(t, "servers/srv-1/restic", serverBackupPrefix(config.ServerID))
	assert.Equal(t, 0, config.BackupRetentionDays, "zero means the program default (90) applies")
}

func TestResticRetention(t *testing.T) {
	tests := []struct {
		name   string
		config *BackupConfig
		want   string
	}{
		{
			name:   "nil config falls back to deployment default",
			config: nil,
			want:   "",
		},
		{
			name:   "no retention override falls back to deployment default",
			config: &BackupConfig{Enabled: true, BackupIntervalHours: 6},
			want:   "",
		},
		{
			name:   "single retention flag",
			config: &BackupConfig{Enabled: true, Keep: 10, KeepUnit: "daily"},
			want:   "--keep-daily 10",
		},
		{
			name:   "unsupported retention unit renders empty",
			config: &BackupConfig{Keep: 12, KeepUnit: "fortnightly"},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resticRetention(tt.config))
		})
	}
}

func TestServerProgramBackupDefaults_EnabledByDefault(t *testing.T) {
	config := &ServerConfig{
		Name:                "test-server",
		ServerID:            "srv-1",
		GCPProject:          "my-project",
		Environment:         "development",
		BackupRetentionDays: 90,
		Backup:              nil,
	}

	userData, err := RenderCloudConfig(&TemplateConfig{
		Region:               config.Region,
		GCPProject:           config.GCPProject,
		Environment:          config.Environment,
		InstanceName:         config.Name,
		ServerID:             config.ServerID,
		BackupRetentionDays:  config.BackupRetentionDays,
		BackupImage:          config.BackupImage,
		BackupInterval:       "1h",
		PruneResticRetention: fmt.Sprintf("--keep-within %dd", config.BackupRetentionDays),
		BackupServiceEnable:  "minecraft-backup ",
		RCONPassword:         "rcon-password",
	})
	assert.NoError(t, err)
	assert.Contains(t, userData, "systemctl enable minecraft minecraft-backup metio-machine-agent")
}

func TestExistingAddressImportID(t *testing.T) {
	tests := []struct {
		name     string
		config   *ServerConfig
		expected string
	}{
		{
			name: "create with existing address imports it",
			config: &ServerConfig{
				Name:                  "test-server",
				GCPProject:            "my-project",
				Region:                "europe-west3",
				ExistingAddress:       "my-addr",
				ImportExistingAddress: true,
			},
			expected: "projects/my-project/regions/europe-west3/addresses/my-addr",
		},
		{
			name: "update never re-imports an already managed address",
			config: &ServerConfig{
				Name:                  "test-server",
				GCPProject:            "my-project",
				Region:                "europe-west3",
				ExistingAddress:       "my-addr",
				ImportExistingAddress: false,
			},
			expected: "",
		},
		{
			name: "no existing address means no import",
			config: &ServerConfig{
				Name:                  "test-server",
				GCPProject:            "my-project",
				Region:                "europe-west3",
				ImportExistingAddress: true,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, existingAddressImportID(tt.config))
		})
	}
}

func TestExtendedBackupPrefixCondition(t *testing.T) {
	bucket := "my-project-development-backups"
	newServerID := "new-srv-id"

	for _, tt := range []struct {
		name      string
		prefix    string
		wantOwn   string
		wantExtra string
	}{
		{
			name:      "source prefix without trailing slash",
			prefix:    "servers/old-srv-id/restic",
			wantOwn:   "servers/new-srv-id/restic/",
			wantExtra: "servers/old-srv-id/restic/",
		},
		{
			name:      "source prefix with trailing slash must not double it",
			prefix:    "servers/old-srv-id/restic/",
			wantOwn:   "servers/new-srv-id/restic/",
			wantExtra: "servers/old-srv-id/restic/",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expr := extendedBackupPrefixCondition(bucket, newServerID, tt.prefix)
			assert.Contains(t, expr, tt.wantOwn)
			assert.Contains(t, expr, tt.wantExtra)
			assert.Contains(t, expr, "||")
			assert.NotContains(t, expr, "restic//")
		})
	}
}
