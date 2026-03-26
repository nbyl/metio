package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var Version = "dev" // default, overridden by ldflags

var execCommand = exec.Command
var getMinecraftPlayerCountFunc = getMinecraftPlayerCount
var getUptimeFunc = getUptime
var getMinecraftVersionFunc = getMinecraftVersion
var osReadFile = os.ReadFile

func main() {
	// Initialize OpenTelemetry
	if err := tracing.InitTracerWithDetails("metio-machine-agent", Version); err != nil {
		log.Printf("Failed to initialize tracer: %v", err)
	}
	if err := tracing.InitMetrics(); err != nil {
		log.Printf("Failed to initialize metrics: %v", err)
	}
	defer tracing.ShutdownTracer()

	viper.SetDefault("MINECRAFT_CHECK_INTERVAL", "30s")
	viper.AutomaticEnv()

	intervalStr := viper.GetString("MINECRAFT_CHECK_INTERVAL")
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Fatalf("Invalid interval format: %v", err)
	}

	environment := viper.GetString("ENVIRONMENT")
	if environment == "" {
		environment = "dev"
	}
	region := viper.GetString("REGION")
	if region == "" {
		region = "us-central1"
	}
	instanceName := viper.GetString("INSTANCE_NAME")
	if instanceName == "" {
		instanceName = "minecraft-server"
	}

	projectID, err := metadata.ProjectID()
	if err != nil {
		log.Fatalf("Error getting project ID: %v", err)
	}

	ctx := context.Background()
	databaseID := fmt.Sprintf("%s-%s-metio-db", environment, region)
	dbConn, err := db.NewConnection(ctx, projectID, databaseID)
	if err != nil {
		log.Fatalf("Error creating Firestore client: %v", err)
	}

	fmt.Printf("Machine agent started with check interval: %v\n", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if err := runStatusUpdate(ctx, dbConn, instanceName); err != nil {
				log.Printf("Error in status update: %v", err)
			}
		}
	}()

	// Keep the program running
	select {}
}

func runStatusUpdate(ctx context.Context, dbConn db.DB, instanceName string) error {
	tracer := otel.Tracer("machine-agent")
	ctx, span := tracer.Start(ctx, "runStatusUpdate")
	defer span.End()

	span.SetAttributes(
		attribute.String("instance.name", instanceName),
	)

	// Record database operation
	tracing.RecordDBOperation("status_update")

	current, max, err := getMinecraftPlayerCountFunc()
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_player_count_failed"))
		tracing.RecordError("get_player_count_failed")
		return err
	}
	uptime, err := getUptimeFunc()
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_uptime_failed"))
		tracing.RecordError("get_uptime_failed")
		return err
	}
	instanceIP, err := getInstanceIP()
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_instance_ip_failed"))
		tracing.RecordError("get_instance_ip_failed")
		log.Printf("Error getting instance IP: %v", err)
		instanceIP = "unknown:25565"
	}
	version, rawOutput, err := getMinecraftVersionFunc()
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_version_failed"))
		tracing.RecordError("get_version_failed")
		log.Printf("Error getting Minecraft version: %v", err)
		version = "Unknown"
	}
	if rawOutput != "" {
		span.SetAttributes(attribute.String("version.raw_output", rawOutput))
		log.Printf("Version parsing failed, raw RCON output: %s", rawOutput)
	}

	span.SetAttributes(
		attribute.Int("players.current", current),
		attribute.Int("players.max", max),
		attribute.String("uptime", uptime),
		attribute.String("instance.ip", instanceIP),
		attribute.String("version", version),
	)

	err = dbConn.UpdateStatus(ctx, instanceName, db.Status{
		Players:     db.Players{Current: current, Max: max},
		Timestamp:   time.Now(),
		Uptime:      uptime,
		ServerState: db.ServerStateRunning,
		InstanceIP:  instanceIP,
		Version:     version,
	})
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		tracing.RecordError("update_status_failed")
		return err
	}

	// Record status update metric
	tracing.RecordStatusUpdate(instanceName, "running")

	span.SetAttributes(attribute.String("success", "true"))
	return nil
}

func getMinecraftPlayerCount() (int, int, error) {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "list")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	// Parse output like "There are 2 of a max of 20 players online: Steve, Alex"
	re := regexp.MustCompile(`There are (\d+) of a max of (\d+) players online`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 3 {
		return 0, 0, fmt.Errorf("could not parse player count from output: %s", string(output))
	}

	current, _ := strconv.Atoi(matches[1])
	max, _ := strconv.Atoi(matches[2])
	return current, max, nil
}

func getUptime() (string, error) {
	data, err := osReadFile("/proc/uptime")
	if err != nil {
		return "", err
	}

	// Parse uptime in seconds from /proc/uptime (first number)
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return "", fmt.Errorf("could not parse uptime from /proc/uptime: %s", string(data))
	}

	uptimeSeconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", fmt.Errorf("could not parse uptime seconds: %v", err)
	}

	// Convert to duration and format
	duration := time.Duration(uptimeSeconds * float64(time.Second))
	return formatDuration(duration), nil
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d days, %d:%02d", days, hours, minutes)
	}
	return fmt.Sprintf("%d:%02d", hours, minutes)
}

func getInstanceIP() (string, error) {
	ip, err := metadata.ExternalIP()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:25565", ip), nil
}

func getMinecraftVersion() (version string, rawOutput string, err error) {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "version")
	output, err := cmd.Output()
	if err != nil {
		// Command failed - return "Unknown" with no raw output
		return "Unknown", "", nil
	}

	outputStr := string(output)

	// Try Paper/Spigot format: "(MC: 1.21.4)"
	paperRe := regexp.MustCompile(`\(MC: ([0-9.]+)\)`)
	if matches := paperRe.FindStringSubmatch(outputStr); len(matches) >= 2 {
		return matches[1], "", nil
	}

	// Try Vanilla RCON format: "name = 1.21.10"
	// Example: "Server version info:id = 1.21.10name = 1.21.10data = 4556..."
	vanillaRconRe := regexp.MustCompile(`name\s*=\s*([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	if matches := vanillaRconRe.FindStringSubmatch(outputStr); len(matches) >= 2 {
		return matches[1], "", nil
	}

	// Fallback: Vanilla log format - look for version pattern like "1.21.4"
	// Example: "Starting minecraft server version 1.21.4"
	vanillaRe := regexp.MustCompile(`version ([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	if matches := vanillaRe.FindStringSubmatch(outputStr); len(matches) >= 2 {
		return matches[1], "", nil
	}

	// Both regexes failed - return "Unknown" with raw output for debugging
	return "Unknown", outputStr, nil
}
