package programs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ServerConfig struct {
	Name              string
	ServerID          string
	Region            string
	Zone              string
	MachineType       string
	MinecraftVersion  string
	DiskSizeGB        int
	Environment       string
	MachineAgentImage string
	BackupImage       string
	GCPProject        string
	RCONPassword      string
	ExistingAddress   string
	ControllerURL     string
	AgentToken        string

	// Backup holds a per-server override for the backup schedule and Restic
	// retention policy. When nil, the deployment defaults apply (backup
	// enabled, hourly interval, keep-within BackupRetentionDays), so existing
	// servers are unaffected until a backup config is set for them.
	Backup *BackupConfig

	// BackupBucket is the deployment-wide central backup bucket (ADR-0004).
	// When empty it is derived from GCPProject and Environment as
	// "{project}-{environment}-backups", matching what the deployment
	// infrastructure provisions.
	BackupBucket string

	// BackupRetentionDays controls how long Restic keeps snapshots on an active
	// server (PRUNE_RESTIC_RETENTION). Defaults to 90 days when unset.
	BackupRetentionDays int

	// BackupResticPassword is the deployment-wide Restic password stored in
	// Secret Manager. It must not reuse the RCON password. When empty it falls
	// back to RCONPassword for local development only.
	BackupResticPassword string

	// RetainLegacyBackupBucket adopts the pre-existing per-server backup bucket
	// with retain-on-delete during the ADR-0004 rollout so the bucket and its
	// snapshots are never destroyed. New stacks must leave it false so no
	// server-owned bucket is created.
	RetainLegacyBackupBucket bool

	// ImportExistingAddress adopts the pre-existing GCP address named by
	// ExistingAddress into the stack instead of creating a new one. It is only
	// set on the initial create; updates must leave it false so that an
	// already-managed address is never re-imported.
	ImportExistingAddress bool
}

// centralBackupBucketName returns the deployment-wide central backup bucket
// name for a project and environment (ADR-0004).
func centralBackupBucketName(projectID, environment string) string {
	return fmt.Sprintf("%s-%s-backups", projectID, environment)
}

// serverBackupPrefix returns the object prefix under which a server's Restic
// repository lives inside the central backup bucket.
func serverBackupPrefix(serverID string) string {
	return fmt.Sprintf("servers/%s/restic", serverID)
}

// backupPrefixCondition returns the IAM condition expression that scopes a
// principal's object access to a single server's Restic prefix inside the
// central bucket (least privilege).
func backupPrefixCondition(bucket, serverID string) string {
	return fmt.Sprintf("resource.name.startsWith(\"projects/_/buckets/%s/objects/%s/\")",
		bucket, serverBackupPrefix(serverID))
}

// existingAddressImportID returns the Pulumi import ID for a pre-existing GCP
// address, or "" when no address should be adopted. Adoption only happens on
// the initial create; on updates the address is already managed by the stack
// and must not be re-imported.
func existingAddressImportID(config *ServerConfig) string {
	if !config.ImportExistingAddress || config.ExistingAddress == "" {
		return ""
	}
	return fmt.Sprintf("projects/%s/regions/%s/addresses/%s",
		config.GCPProject, config.Region, config.ExistingAddress)
}

// BackupConfig holds a per-server override for the backup schedule and Restic
// retention policy. Zero values fall back to the deployment defaults.
type BackupConfig struct {
	Enabled        bool
	BackupSchedule string
	KeepLast       int
	KeepHourly     int
	KeepDaily      int
	KeepWeekly     int
	KeepMonthly    int
	KeepYearly     int
}

// resticRetention renders the PRUNE_RESTIC_RETENTION argument for a backup
// config, or "" when no retention override is set so the deployment default
// (keep-within BackupRetentionDays) is used.
func resticRetention(b *BackupConfig) string {
	if b == nil {
		return ""
	}
	args := []string{}
	flags := []struct {
		name  string
		value int
	}{
		{"--keep-last", b.KeepLast},
		{"--keep-hourly", b.KeepHourly},
		{"--keep-daily", b.KeepDaily},
		{"--keep-weekly", b.KeepWeekly},
		{"--keep-monthly", b.KeepMonthly},
		{"--keep-yearly", b.KeepYearly},
	}
	for _, f := range flags {
		if f.value > 0 {
			args = append(args, fmt.Sprintf("%s %d", f.name, f.value))
		}
	}
	return strings.Join(args, " ")
}

func ServerProgram(config *ServerConfig) func(*pulumi.Context) error {
	return func(ctx *pulumi.Context) error {
		if config.Name == "" {
			return fmt.Errorf("server name is required")
		}
		if config.Region == "" {
			config.Region = "europe-west3"
		}
		if config.Zone == "" {
			config.Zone = "europe-west3-a"
		}
		if config.MachineType == "" {
			config.MachineType = "e2-micro"
		}
		if config.DiskSizeGB == 0 {
			config.DiskSizeGB = 10
		}
		if config.Environment == "" {
			config.Environment = "development"
		}
		if config.RCONPassword == "" {
			config.RCONPassword = "minecraft2025"
		}
		if config.BackupBucket == "" {
			config.BackupBucket = centralBackupBucketName(config.GCPProject, config.Environment)
		}
		if config.BackupRetentionDays == 0 {
			config.BackupRetentionDays = 90
		}
		resticPassword := config.BackupResticPassword
		if resticPassword == "" {
			resticPassword = config.RCONPassword
		}

		backupImage := config.BackupImage
		if backupImage == "" {
			backupImage = "ghcr.io/itzg/mc-backup:latest"
		}

		// Backup scheduling and retention. Per-server overrides take effect
		// only when a backup config is provided; otherwise the deployment
		// defaults apply so existing servers keep their current behavior.
		backupEnabled := true
		backupInterval := "1h"
		pruneResticRetention := fmt.Sprintf("--keep-within %dd", config.BackupRetentionDays)
		backupServiceEnable := "minecraft-backup "
		if config.Backup != nil {
			backupEnabled = config.Backup.Enabled
			if config.Backup.BackupSchedule != "" {
				backupInterval = config.Backup.BackupSchedule
			}
			if retention := resticRetention(config.Backup); retention != "" {
				pruneResticRetention = retention
			}
			if !backupEnabled {
				backupServiceEnable = ""
			}
		}

		userData, err := RenderCloudConfig(&TemplateConfig{
			Region:               config.Region,
			GCPProject:           config.GCPProject,
			Environment:          config.Environment,
			InstanceName:         config.Name,
			BackupBucket:         config.BackupBucket,
			ServerID:             config.ServerID,
			BackupRetentionDays:  config.BackupRetentionDays,
			ResticPassword:       resticPassword,
			MachineAgentImage:    config.MachineAgentImage,
			BackupImage:          backupImage,
			BackupInterval:       backupInterval,
			PruneResticRetention: pruneResticRetention,
			BackupServiceEnable:  backupServiceEnable,
			MinecraftVersion:     config.MinecraftVersion,
			RCONPassword:         config.RCONPassword,
			ControllerURL:        config.ControllerURL,
			AgentToken:           config.AgentToken,
		})
		if err != nil {
			return fmt.Errorf("failed to generate cloud-config: %w", err)
		}

		// Compute a hash of the cloud-config to detect changes that require VM recreation.
		h := sha256.New()
		// The digest is a change-detection fingerprint for the cloud_config_hash
		// label (ReplaceOnChanges); no credential is hashed for storage or
		// authentication, so SHA-256 (a fast hash) is appropriate here.
		// codeql[go/weak-sensitive-data-hashing]
		h.Write([]byte(userData))
		cloudConfigHash := hex.EncodeToString(h.Sum(nil))[:16] // first 16 hex chars

		sa, err := serviceaccount.NewAccount(ctx, fmt.Sprintf("%s-sa", config.Name), &serviceaccount.AccountArgs{
			AccountId:   pulumi.String(fmt.Sprintf("%s-sa", config.Name)),
			DisplayName: pulumi.String(fmt.Sprintf("VM service account for %s", config.Name)),
		})
		if err != nil {
			return fmt.Errorf("failed to create service account: %w", err)
		}

		iamRoles := []string{
			"roles/logging.logWriter",
			"roles/cloudtrace.agent",
			"roles/artifactregistry.reader",
			"roles/serviceusage.serviceUsageConsumer",
		}

		for _, role := range iamRoles {
			roleName := strings.ReplaceAll(role, "roles/", "")
			_, err = projects.NewIAMMember(ctx, fmt.Sprintf("%s-%s", config.Name, roleName), &projects.IAMMemberArgs{
				Project: pulumi.String(config.GCPProject),
				Role:    pulumi.String(role),
				Member:  pulumi.Sprintf("serviceAccount:%s", sa.Email),
			})
			if err != nil {
				return fmt.Errorf("failed to create IAM binding for %s: %w", role, err)
			}
		}

		if config.RetainLegacyBackupBucket {
			// ADR-0004 migration: adopt the pre-existing per-server bucket with
			// retain-on-delete so the first central-backup rollout never
			// destroys existing snapshots. The bucket is kept managed but
			// retained; a later cleanup step can drop this resource safely.
			legacyBucketName := fmt.Sprintf("%s-%s-backups", config.GCPProject, config.Name)
			_, err = storage.NewBucket(ctx, fmt.Sprintf("%s-backup-bucket", config.Name), &storage.BucketArgs{
				Name:                     pulumi.String(legacyBucketName),
				Location:                 pulumi.String(config.Region),
				UniformBucketLevelAccess: pulumi.Bool(true),
				ForceDestroy:             pulumi.Bool(true),
			}, pulumi.RetainOnDelete(true))
			if err != nil {
				return fmt.Errorf("failed to retain legacy backup bucket: %w", err)
			}
		}

		// Grant the VM service account object access scoped to this server's
		// Restic prefix inside the central bucket (least privilege).
		_, err = storage.NewBucketIAMMember(ctx, fmt.Sprintf("%s-backup-bucket-admin", config.Name), &storage.BucketIAMMemberArgs{
			Bucket: pulumi.String(config.BackupBucket),
			Role:   pulumi.String("roles/storage.objectAdmin"),
			Member: pulumi.Sprintf("serviceAccount:%s", sa.Email),
			Condition: &storage.BucketIAMMemberConditionArgs{
				Title:       pulumi.String("server-backup-prefix"),
				Description: pulumi.String("Limit access to this server's Restic repository prefix"),
				Expression:  pulumi.String(backupPrefixCondition(config.BackupBucket, config.ServerID)),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to grant backup bucket access: %w", err)
		}

		disk, err := compute.NewDisk(ctx, fmt.Sprintf("%s-disk", config.Name), &compute.DiskArgs{
			Name: pulumi.String(fmt.Sprintf("%s-data", config.Name)),
			Type: pulumi.String(fmt.Sprintf("zones/%s/diskTypes/pd-standard", config.Zone)),
			Size: pulumi.Int(config.DiskSizeGB),
			Zone: pulumi.String(config.Zone),
		})
		if err != nil {
			return fmt.Errorf("failed to create disk: %w", err)
		}

		addressGCPName := fmt.Sprintf("%s-addr", config.Name)
		if config.ExistingAddress != "" {
			addressGCPName = config.ExistingAddress
		}

		addressOpts := []pulumi.ResourceOption{}
		if importID := existingAddressImportID(config); importID != "" {
			addressOpts = append(addressOpts, pulumi.Import(pulumi.ID(importID)))
		}

		address, err := compute.NewAddress(ctx, fmt.Sprintf("%s-address", config.Name), &compute.AddressArgs{
			Name:   pulumi.String(addressGCPName),
			Region: pulumi.String(config.Region),
		}, addressOpts...)
		if err != nil {
			return fmt.Errorf("failed to create address: %w", err)
		}

		firewall, err := compute.NewFirewall(ctx, fmt.Sprintf("%s-firewall", config.Name), &compute.FirewallArgs{
			Name:    pulumi.String(fmt.Sprintf("%s-fw", config.Name)),
			Network: pulumi.String("default"),
			Allows: compute.FirewallAllowArray{
				&compute.FirewallAllowArgs{
					Protocol: pulumi.String("icmp"),
				},
				&compute.FirewallAllowArgs{
					Protocol: pulumi.String("tcp"),
					Ports: pulumi.StringArray{
						pulumi.String("25565"),
					},
				},
			},
			SourceRanges: pulumi.StringArray{
				pulumi.String("0.0.0.0/0"),
			},
			TargetTags: pulumi.StringArray{
				pulumi.String(config.Name),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create firewall: %w", err)
		}

		instance, err := compute.NewInstance(ctx, config.Name, &compute.InstanceArgs{
			Name:        pulumi.String(config.Name),
			MachineType: pulumi.String(config.MachineType),
			Zone:        pulumi.String(config.Zone),
			BootDisk: &compute.InstanceBootDiskArgs{
				InitializeParams: &compute.InstanceBootDiskInitializeParamsArgs{
					Image: pulumi.String("cos-cloud/cos-stable"),
				},
			},
			AttachedDisks: compute.InstanceAttachedDiskArray{
				&compute.InstanceAttachedDiskArgs{
					Source:     disk.Name,
					Mode:       pulumi.String("READ_WRITE"),
					DeviceName: pulumi.String("minecraft-data"),
				},
			},
			NetworkInterfaces: compute.InstanceNetworkInterfaceArray{
				&compute.InstanceNetworkInterfaceArgs{
					Network: pulumi.String("default"),
					AccessConfigs: compute.InstanceNetworkInterfaceAccessConfigArray{
						&compute.InstanceNetworkInterfaceAccessConfigArgs{
							NatIp: address.Address,
						},
					},
				},
			},
			Scheduling: &compute.InstanceSchedulingArgs{
				Preemptible:       pulumi.Bool(true),
				AutomaticRestart:  pulumi.Bool(false),
				ProvisioningModel: pulumi.String("SPOT"),
			},
			Tags: pulumi.StringArray{
				pulumi.String(config.Name),
				pulumi.String(config.Environment),
			},
			ServiceAccount: &compute.InstanceServiceAccountArgs{
				Email: sa.Email,
				Scopes: pulumi.StringArray{
					pulumi.String("cloud-platform"),
				},
			},
			Labels: pulumi.StringMap{
				"cloud_config_hash": pulumi.String(cloudConfigHash),
				"infra_version":     pulumi.String(fmt.Sprintf("%d", CurrentInfraVersion)),
				"server_id":         pulumi.String(config.ServerID),
			},
			Metadata: pulumi.StringMap{
				"user-data": pulumi.String(userData),
			},
		}, pulumi.ReplaceOnChanges([]string{"labels.cloud_config_hash"}), pulumi.DeleteBeforeReplace(true), pulumi.DependsOn([]pulumi.Resource{firewall}))
		if err != nil {
			return fmt.Errorf("failed to create instance: %w", err)
		}

		shutdownPolicy, err := compute.NewResourcePolicy(ctx, fmt.Sprintf("%s-daily-shutdown", config.Name), &compute.ResourcePolicyArgs{
			Name:   pulumi.String(fmt.Sprintf("%s-shutdown", config.Name)),
			Region: pulumi.String(config.Region),
			InstanceSchedulePolicy: &compute.ResourcePolicyInstanceSchedulePolicyArgs{
				TimeZone: pulumi.String("Europe/Berlin"),
				VmStopSchedule: &compute.ResourcePolicyInstanceSchedulePolicyVmStopScheduleArgs{
					Schedule: pulumi.String("0 21 * * *"),
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create shutdown policy: %w", err)
		}

		_, err = compute.NewResourcePolicyAttachment(ctx, fmt.Sprintf("%s-shutdown-attachment", config.Name), &compute.ResourcePolicyAttachmentArgs{
			Name:     shutdownPolicy.Name,
			Instance: instance.Name,
			Zone:     pulumi.String(config.Zone),
		})
		if err != nil {
			return fmt.Errorf("failed to attach shutdown policy: %w", err)
		}

		ctx.Export("instanceName", instance.Name)
		ctx.Export("instanceIP", instance.NetworkInterfaces.ApplyT(func(nics []compute.InstanceNetworkInterface) string {
			if len(nics) == 0 {
				return ""
			}
			nic := nics[0]
			if len(nic.AccessConfigs) == 0 {
				return ""
			}
			if nic.AccessConfigs[0].NatIp == nil {
				return ""
			}
			return *nic.AccessConfigs[0].NatIp
		}))
		ctx.Export("zone", pulumi.String(config.Zone))
		ctx.Export("diskName", disk.Name)
		ctx.Export("serviceAccount", sa.Email)
		ctx.Export("backupBucket", pulumi.String(config.BackupBucket))
		ctx.Export("backupPrefix", pulumi.String(serverBackupPrefix(config.ServerID)))

		return nil
	}
}
