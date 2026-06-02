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
	GCPProject        string
	RCONPassword      string
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

		backupBucketName := fmt.Sprintf("%s-%s-backups", config.GCPProject, config.Name)

		userData, err := RenderCloudConfig(&TemplateConfig{
			Region:            config.Region,
			GCPProject:        config.GCPProject,
			Environment:       config.Environment,
			InstanceName:      config.Name,
			BackupBucket:      backupBucketName,
			MachineAgentImage: config.MachineAgentImage,
			MinecraftVersion:  config.MinecraftVersion,
			RCONPassword:      config.RCONPassword,
		})
		if err != nil {
			return fmt.Errorf("failed to generate cloud-config: %w", err)
		}

		// Compute a hash of the cloud-config to detect changes that require VM recreation.
		h := sha256.New()
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
			"roles/storage.objectUser",
			"roles/storage.objectCreator",
			"roles/logging.logWriter",
			"roles/monitoring.metricWriter",
			"roles/cloudtrace.agent",
			"roles/artifactregistry.reader",
			"roles/datastore.user",
			"roles/serviceusage.serviceUsageConsumer",
			"roles/compute.instanceAdmin.v1",
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

		backupBucket, err := storage.NewBucket(ctx, fmt.Sprintf("%s-backup-bucket", config.Name), &storage.BucketArgs{
			Name:                     pulumi.String(backupBucketName),
			Location:                 pulumi.String(config.Region),
			UniformBucketLevelAccess: pulumi.Bool(true),
			ForceDestroy:             pulumi.Bool(false),
		})
		if err != nil {
			return fmt.Errorf("failed to create backup bucket: %w", err)
		}

		_, err = storage.NewBucketIAMMember(ctx, fmt.Sprintf("%s-backup-bucket-admin", config.Name), &storage.BucketIAMMemberArgs{
			Bucket: backupBucket.Name,
			Role:   pulumi.String("roles/storage.objectAdmin"),
			Member: pulumi.Sprintf("serviceAccount:%s", sa.Email),
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

		address, err := compute.NewAddress(ctx, fmt.Sprintf("%s-address", config.Name), &compute.AddressArgs{
			Name:   pulumi.String(fmt.Sprintf("%s-addr", config.Name)),
			Region: pulumi.String(config.Region),
		})
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
				pulumi.String(config.ServerID),
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
			},
			Metadata: pulumi.StringMap{
				"user-data": pulumi.String(userData),
			},
		}, pulumi.ReplaceOnChanges([]string{"labels.cloud_config_hash"}), pulumi.DependsOn([]pulumi.Resource{firewall}))
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
		ctx.Export("backupBucket", pulumi.String(backupBucketName))

		return nil
	}
}
