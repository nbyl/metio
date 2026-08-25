package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/nbyl/metio/internal/agentclient"
	"github.com/nbyl/metio/internal/dbtypes"
	"github.com/nbyl/metio/internal/tracing"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var Version = "dev"

var execCommand = exec.Command
var getMinecraftPlayerCountFunc = getMinecraftPlayerCount
var getUptimeFunc = getUptime
var getMinecraftVersionFunc = getMinecraftVersion
var osReadFile = os.ReadFile
var syncWhitelistFunc = syncWhitelist
var importWhitelistIfEmptyFunc = importWhitelistIfEmpty
var checkScheduledShutdownFunc = checkScheduledShutdown
var getInstanceIPFunc = getInstanceIP
var sendMinecraftMessageFunc = sendMinecraftMessage
var saveMinecraftWorldFunc = saveMinecraftWorld
var getProjectIDFunc = getProjectID
var getZoneFunc = getZone
var handlePendingCommandFunc = func(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
	return nil
}

type WarningState int

const (
	WarningStateNone WarningState = iota
	WarningStateFiveMin
	WarningStateOneMin
)

var shutdownWarningState WarningState
var lastScheduledShutdownTime *time.Time

func main() {
	viper.SetDefault("MINECRAFT_CHECK_INTERVAL", "30s")
	viper.AutomaticEnv()

	if err := tracing.InitTracerWithDetails("metio-machine-agent", Version); err != nil {
		log.Printf("Failed to initialize tracer: %v", err)
	}
	if err := tracing.InitMetrics(); err != nil {
		log.Printf("Failed to initialize metrics: %v", err)
	}
	defer tracing.ShutdownTracer()

	intervalStr := viper.GetString("MINECRAFT_CHECK_INTERVAL")
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Fatalf("Invalid interval format: %v", err)
	}

	controllerURL := os.Getenv("CONTROLLER_URL")
	if controllerURL == "" {
		log.Fatal("CONTROLLER_URL must be set")
	}
	agentToken := os.Getenv("AGENT_TOKEN")
	if agentToken == "" {
		log.Fatal("AGENT_TOKEN must be set")
	}

	instanceName, err := getInstanceName()
	if err != nil {
		log.Fatalf("Error getting instance name from metadata: %v", err)
	}

	ctx := context.Background()
	client := agentclient.New(controllerURL, agentToken, instanceName)

	fmt.Printf("Machine agent started with check interval: %v\n", interval)

	handlePendingCommandFunc = handlePendingCommand

	if err := importWhitelistIfEmptyFunc(ctx, client, instanceName); err != nil {
		log.Printf("Error during initial whitelist import: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if err := runStatusUpdate(ctx, client, instanceName); err != nil {
				log.Printf("Error in status update: %v", err)
			}
			if err := processBackupManifestsFunc(ctx, client); err != nil {
				log.Printf("Error processing backup manifests: %v", err)
			}
		}
	}()

	select {}
}

func runStatusUpdate(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
	tracer := otel.Tracer("machine-agent")
	ctx, span := tracer.Start(ctx, "runStatusUpdate")
	defer span.End()

	span.SetAttributes(
		attribute.String("instance.name", instanceName),
	)

	if err := checkScheduledShutdownFunc(ctx, client, instanceName); err != nil {
		span.SetAttributes(attribute.String("error", "check_scheduled_shutdown_failed"))
		log.Printf("Error checking scheduled shutdown: %v", err)
	}

	if err := handlePendingCommandFunc(ctx, client, instanceName); err != nil {
		span.SetAttributes(attribute.String("error", "handle_pending_command_failed"))
		log.Printf("Error handling pending command: %v", err)
	}

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
	instanceIP, err := getInstanceIPFunc()
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

	whitelistEnabled, err := syncWhitelistFunc(ctx, client, instanceName)
	if err != nil {
		span.SetAttributes(attribute.String("error", "sync_whitelist_failed"))
		log.Printf("Error syncing whitelist: %v", err)
		whitelistEnabled = false
	}
	span.SetAttributes(attribute.Bool("whitelist.enabled", whitelistEnabled))

	currentStatus, _ := client.GetStatus(ctx)

	err = client.UpdateStatus(ctx, dbtypes.Status{
		Players:           dbtypes.Players{Current: current, Max: max},
		Timestamp:         time.Now(),
		Uptime:            uptime,
		ServerState:       dbtypes.ServerStateRunning,
		InstanceIP:        instanceIP,
		Version:           version,
		WhitelistEnabled:  whitelistEnabled,
		ScheduledShutdown: currentStatus.ScheduledShutdown,
		AgentVersion:      Version,
	})
	if err != nil {
		span.SetAttributes(attribute.String("error", "update_status_failed"))
		tracing.RecordError("update_status_failed")
		return err
	}

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

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return "", fmt.Errorf("could not parse uptime from /proc/uptime: %s", string(data))
	}

	uptimeSeconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", fmt.Errorf("could not parse uptime seconds: %v", err)
	}

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

func getProjectID() (string, error) {
	return metadata.ProjectID()
}

func getZone() (string, error) {
	return metadata.Zone()
}

func getInstanceName() (string, error) {
	return metadata.InstanceName()
}

func getMinecraftVersion() (version string, rawOutput string, err error) {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "version")
	output, err := cmd.Output()
	if err != nil {
		return "Unknown", "", nil
	}

	outputStr := string(output)

	paperRe := regexp.MustCompile(`\(MC: ([0-9.]+)\)`)
	if matches := paperRe.FindStringSubmatch(outputStr); len(matches) >= 2 {
		return matches[1], "", nil
	}

	vanillaRconRe := regexp.MustCompile(`name\s*=\s*([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	if matches := vanillaRconRe.FindStringSubmatch(outputStr); len(matches) >= 2 {
		return matches[1], "", nil
	}

	vanillaRe := regexp.MustCompile(`version ([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	if matches := vanillaRe.FindStringSubmatch(outputStr); len(matches) >= 2 {
		return matches[1], "", nil
	}

	return "Unknown", outputStr, nil
}

type MinecraftWhitelistEntry struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

func importWhitelistIfEmpty(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
	entries, err := client.GetWhitelistEntries(ctx)
	if err != nil {
		log.Printf("Error checking whitelist entries: %v", err)
	}

	if len(entries) > 0 {
		log.Printf("Controller whitelist already has %d entries, skipping import", len(entries))
		return nil
	}

	log.Println("Controller whitelist is empty, attempting to import from whitelist.json")

	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "cat", "/data/whitelist.json")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read whitelist.json: %w", err)
	}

	var mcEntries []MinecraftWhitelistEntry
	if err := json.Unmarshal(output, &mcEntries); err != nil {
		return fmt.Errorf("failed to parse whitelist.json: %w", err)
	}

	if len(mcEntries) == 0 {
		log.Println("whitelist.json is empty, nothing to import")
		return nil
	}

	for _, mcEntry := range mcEntries {
		entry := dbtypes.WhitelistEntry{
			Username: mcEntry.Name,
			UUID:     mcEntry.UUID,
			AddedAt:  time.Now(),
			AddedBy:  "imported",
		}
		if err := client.AddWhitelistEntry(ctx, entry); err != nil {
			log.Printf("Error importing whitelist entry %s: %v", mcEntry.Name, err)
			continue
		}
		log.Printf("Imported whitelist entry: %s (%s)", mcEntry.Name, mcEntry.UUID)
	}

	whitelistEnabled := getWhitelistEnabledStatus()
	if err := client.SetWhitelistConfig(ctx, dbtypes.WhitelistConfig{Enabled: whitelistEnabled}); err != nil {
		log.Printf("Error setting whitelist config: %v", err)
	}

	log.Printf("Imported %d whitelist entries from whitelist.json", len(mcEntries))
	return nil
}

func syncWhitelist(ctx context.Context, client agentclient.AgentClient, instanceName string) (bool, error) {
	config, err := client.GetWhitelistConfig(ctx)
	if err != nil {
		config = dbtypes.WhitelistConfig{Enabled: false}
	}

	entries, err := client.GetWhitelistEntries(ctx)
	if err != nil {
		return config.Enabled, fmt.Errorf("failed to get whitelist entries: %w", err)
	}

	minecraftEntries, err := getMinecraftWhitelist()
	if err != nil {
		return config.Enabled, fmt.Errorf("failed to get minecraft whitelist: %w", err)
	}

	entriesMap := make(map[string]dbtypes.WhitelistEntry)
	for _, entry := range entries {
		entriesMap[entry.UUID] = entry
	}

	minecraftMap := make(map[string]bool)
	for _, entry := range minecraftEntries {
		minecraftMap[entry.UUID] = true
	}

	for uuid, entry := range entriesMap {
		if !minecraftMap[uuid] {
			if err := addPlayerToMinecraftWhitelist(entry.Username); err != nil {
				log.Printf("Error adding %s to Minecraft whitelist: %v", entry.Username, err)
			} else {
				log.Printf("Added %s to Minecraft whitelist", entry.Username)
			}
		}
	}

	for _, mcEntry := range minecraftEntries {
		if _, exists := entriesMap[mcEntry.UUID]; !exists {
			if err := removePlayerFromMinecraftWhitelist(mcEntry.Name); err != nil {
				log.Printf("Error removing %s from Minecraft whitelist: %v", mcEntry.Name, err)
			} else {
				log.Printf("Removed %s from Minecraft whitelist", mcEntry.Name)
			}
		}
	}

	currentEnabled := getWhitelistEnabledStatus()
	if config.Enabled != currentEnabled {
		if err := setWhitelistEnabled(config.Enabled); err != nil {
			log.Printf("Error setting whitelist enabled to %v: %v", config.Enabled, err)
		} else {
			log.Printf("Set whitelist enabled to %v", config.Enabled)
		}
	}

	return config.Enabled, nil
}

func getMinecraftWhitelist() ([]MinecraftWhitelistEntry, error) {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "cat", "/data/whitelist.json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read whitelist.json: %w", err)
	}

	var entries []MinecraftWhitelistEntry
	if err := json.Unmarshal(output, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse whitelist.json: %w", err)
	}

	return entries, nil
}

func addPlayerToMinecraftWhitelist(username string) error {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "whitelist", "add", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcon whitelist add failed: %w, output: %s", err, string(output))
	}
	return nil
}

func removePlayerFromMinecraftWhitelist(username string) error {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "whitelist", "remove", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcon whitelist remove failed: %w, output: %s", err, string(output))
	}
	return nil
}

func getWhitelistEnabledStatus() bool {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "cat", "/data/server.properties")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error reading server.properties: %v", err)
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "white-list=") {
			return strings.TrimPrefix(line, "white-list=") == "true"
		}
		if strings.HasPrefix(line, "enforce-whitelist=") {
			return strings.TrimPrefix(line, "enforce-whitelist=") == "true"
		}
	}

	return false
}

func setWhitelistEnabled(enabled bool) error {
	var cmd *exec.Cmd
	if enabled {
		cmd = execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "whitelist", "on")
	} else {
		cmd = execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "whitelist", "off")
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcon whitelist on/off failed: %w, output: %s", err, string(output))
	}
	return nil
}

func checkScheduledShutdown(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
	currentStatus, err := client.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if currentStatus.ScheduledShutdown == nil {
		if lastScheduledShutdownTime != nil {
			log.Println("Scheduled shutdown was cancelled, resetting warning state")
			shutdownWarningState = WarningStateNone
			lastScheduledShutdownTime = nil
		}
		return nil
	}

	shutdownTime := *currentStatus.ScheduledShutdown

	if lastScheduledShutdownTime == nil || !lastScheduledShutdownTime.Equal(shutdownTime) {
		log.Printf("New scheduled shutdown detected: %v", shutdownTime)
		shutdownWarningState = WarningStateNone
		lastScheduledShutdownTime = &shutdownTime
	}

	remaining := time.Until(shutdownTime)

	if remaining <= 0 {
		log.Println("Scheduled shutdown time reached, initiating shutdown...")
		return initiateScheduledShutdown(ctx, client, instanceName)
	}

	if remaining <= 1*time.Minute && shutdownWarningState < WarningStateOneMin {
		if err := sendMinecraftMessageFunc("Server will shut down in 1 minute! Save your progress!"); err != nil {
			log.Printf("Error sending 1-minute warning: %v", err)
		} else {
			log.Println("Sent 1-minute shutdown warning")
		}
		shutdownWarningState = WarningStateOneMin
	} else if remaining <= 5*time.Minute && shutdownWarningState < WarningStateFiveMin {
		if err := sendMinecraftMessageFunc("Server will shut down in 5 minutes!"); err != nil {
			log.Printf("Error sending 5-minute warning: %v", err)
		} else {
			log.Println("Sent 5-minute shutdown warning")
		}
		shutdownWarningState = WarningStateFiveMin
	}

	return nil
}

func sendMinecraftMessage(message string) error {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "say", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcon say failed: %w, output: %s", err, string(output))
	}
	return nil
}

func saveMinecraftWorld() error {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "save-all")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcon save-all failed: %w, output: %s", err, string(output))
	}
	log.Println("World saved successfully")
	return nil
}

func handlePendingCommand(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
	status, err := client.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}
	if status.PendingCommand == "" {
		return nil
	}

	command := status.PendingCommand
	switch command {
	case "save":
		if err := saveMinecraftWorldFunc(); err != nil {
			status.PendingCommandResult = "failed: " + err.Error()
		} else {
			status.PendingCommandResult = "completed"
		}
	default:
		status.PendingCommandResult = "failed: unknown command: " + command
	}
	status.PendingCommand = ""

	return client.UpdateStatus(ctx, status)
}

func initiateScheduledShutdown(ctx context.Context, client agentclient.AgentClient, instanceName string) error {
	if err := sendMinecraftMessageFunc("Server is shutting down NOW!"); err != nil {
		log.Printf("Error sending final shutdown message: %v", err)
	}

	if err := saveMinecraftWorldFunc(); err != nil {
		log.Printf("Error saving world before shutdown: %v", err)
	}

	currentStatus, err := client.GetStatus(ctx)
	if err != nil {
		log.Printf("Error getting status to clear scheduled shutdown: %v", err)
	} else {
		currentStatus.ScheduledShutdown = nil
		currentStatus.Timestamp = time.Now()
		if err := client.UpdateStatus(ctx, currentStatus); err != nil {
			log.Printf("Error clearing scheduled shutdown: %v", err)
		}
	}

	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	time.Sleep(5 * time.Second)

	project, err := getProjectIDFunc()
	if err != nil {
		return fmt.Errorf("failed to get project ID: %w", err)
	}
	zone, err := getZoneFunc()
	if err != nil {
		return fmt.Errorf("failed to get zone: %w", err)
	}

	log.Println("Initiating VM shutdown via controller API...")
	if err := client.StopInstance(ctx, project, zone); err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	return nil
}
