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

var execCommand = exec.Command
var getMinecraftPlayerCountFunc = getMinecraftPlayerCount
var getUptimeFunc = getUptime
var osReadFile = os.ReadFile

func main() {
	// Initialize OpenTelemetry
	if err := tracing.InitTracerWithDetails("metio-machine-agent", "1.0.0"); err != nil {
		log.Printf("Failed to initialize tracer: %v", err)
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

	current, max, err := getMinecraftPlayerCountFunc()
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_player_count_failed"))
		return err
	}
	uptime, err := getUptimeFunc()
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_uptime_failed"))
		return err
	}
	instanceIP, err := getInstanceIP()
	if err != nil {
		span.SetAttributes(attribute.String("error", "get_instance_ip_failed"))
		log.Printf("Error getting instance IP: %v", err)
		instanceIP = "unknown:25565"
	}

	span.SetAttributes(
		attribute.Int("players.current", current),
		attribute.Int("players.max", max),
		attribute.String("uptime", uptime),
		attribute.String("instance.ip", instanceIP),
	)

	err = dbConn.UpdateStatus(ctx, instanceName, db.Status{
		Players:     db.Players{Current: current, Max: max},
		Timestamp:   time.Now(),
		Uptime:      uptime,
		ServerState: db.ServerStateRunning,
		InstanceIP:  instanceIP,
	})
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		return err
	}

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
