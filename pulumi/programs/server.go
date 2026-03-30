package programs

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ServerConfig struct {
	Name              string
	Region            string
	Zone              string
	MachineType       string
	MinecraftVersion  string
	DiskSizeGB        int
	Environment       string
	BackupBucket      string
	MachineAgentImage string
	GCPProject        string
	InstanceName      string
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
		if config.InstanceName == "" {
			config.InstanceName = fmt.Sprintf("%s-minecraft-server", config.Environment)
		}
		if config.RCONPassword == "" {
			config.RCONPassword = "minecraft2025"
		}

		userData, err := RenderCloudConfig(&TemplateConfig{
			Region:            config.Region,
			GCPProject:        config.GCPProject,
			Environment:       config.Environment,
			InstanceName:      config.InstanceName,
			BackupBucket:      config.BackupBucket,
			MachineAgentImage: config.MachineAgentImage,
			MinecraftVersion:  config.MinecraftVersion,
			RCONPassword:      config.RCONPassword,
		})
		if err != nil {
			return fmt.Errorf("failed to generate cloud-config: %w", err)
		}

		sa, err := serviceaccount.NewAccount(ctx, fmt.Sprintf("%s-sa", config.Name), &serviceaccount.AccountArgs{
			AccountId:   pulumi.String(fmt.Sprintf("%s-vm-sa", config.Environment)),
			DisplayName: pulumi.String("Custom SA for VM Instance"),
		})
		if err != nil {
			return fmt.Errorf("failed to create service account: %w", err)
		}

		_, err = storage.NewBucketIAMMember(ctx, fmt.Sprintf("%s-backup-bucket-reader", config.Name), &storage.BucketIAMMemberArgs{
			Bucket: pulumi.String(config.BackupBucket),
			Role:   pulumi.String("roles/storage.objectViewer"),
			Member: sa.Email,
		})
		if err != nil {
			return fmt.Errorf("failed to grant backup bucket access: %w", err)
		}

		disk, err := compute.NewDisk(ctx, fmt.Sprintf("%s-disk", config.Name), &compute.DiskArgs{
			Name: pulumi.String(fmt.Sprintf("%s-minecraft-data", config.Environment)),
			Type: pulumi.String(fmt.Sprintf("zones/%s/diskTypes/pd-standard", config.Zone)),
			Size: pulumi.Int(config.DiskSizeGB),
			Zone: pulumi.String(config.Zone),
		})
		if err != nil {
			return fmt.Errorf("failed to create disk: %w", err)
		}

		address, err := compute.NewAddress(ctx, fmt.Sprintf("%s-address", config.Name), &compute.AddressArgs{
			Name:   pulumi.String(fmt.Sprintf("%s-minecraft-server", config.Environment)),
			Region: pulumi.String(config.Region),
		})
		if err != nil {
			return fmt.Errorf("failed to create address: %w", err)
		}

		firewall, err := compute.NewFirewall(ctx, fmt.Sprintf("%s-firewall", config.Name), &compute.FirewallArgs{
			Name:    pulumi.String(fmt.Sprintf("%s-minecraft-server", config.Environment)),
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
				pulumi.String(fmt.Sprintf("%s-minecraft-server", config.Environment)),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create firewall: %w", err)
		}

		instance, err := compute.NewInstance(ctx, config.Name, &compute.InstanceArgs{
			Name:        pulumi.String(fmt.Sprintf("%s-minecraft-server", config.Environment)),
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
				pulumi.String(fmt.Sprintf("%s-minecraft-server", config.Environment)),
				pulumi.String(config.Environment),
			},
			ServiceAccount: &compute.InstanceServiceAccountArgs{
				Email: sa.Email,
				Scopes: pulumi.StringArray{
					pulumi.String("cloud-platform"),
				},
			},
			Metadata: pulumi.StringMap{
				"user-data": pulumi.String(userData),
			},
		}, pulumi.DependsOn([]pulumi.Resource{firewall}))
		if err != nil {
			return fmt.Errorf("failed to create instance: %w", err)
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
		ctx.Export("diskName", disk.Name)
		ctx.Export("serviceAccount", sa.Email)

		return nil
	}
}
