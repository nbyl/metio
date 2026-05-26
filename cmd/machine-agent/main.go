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

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/compute/metadata"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var Version = "dev" // default, overridden by ldflags

var execCommand = exec.Command
var getMinecraftPlayerCountFunc = getMinecraftPlayerCount
var getUptimeFunc = getUptime
var getMinecraftVersionFunc = getMinecraftVersion
var osReadFile = os.ReadFile
var syncWhitelistFunc = syncWhitelist
var importWhitelistIfEmptyFunc = importWhitelistIfEmpty
var checkScheduledShutdownFunc = checkScheduledShutdown
var getInstanceIPFunc = getInstanceIP
var stopInstanceFunc = stopInstance
var sendMinecraftMessageFunc = sendMinecraftMessage
var saveMinecraftWorldFunc = saveMinecraftWorld
var handlePendingCommandFunc = func(ctx context.Context, dbConn db.DB, instanceName string) error {
	return nil // default no-op; production main() replaces with handlePendingCommand
}

// WarningState represents the state of shutdown warnings sent
type WarningState int

const (
	WarningStateNone    WarningState = iota // No warnings sent
	WarningStateFiveMin                     // 5-minute warning sent
	WarningStateOneMin                      // 1-minute warning sent
)

// shutdownWarningState tracks which warnings have been sent for the current scheduled shutdown
var shutdownWarningState WarningState

// lastScheduledShutdownTime tracks the last scheduled shutdown time to detect changes
var lastScheduledShutdownTime *time.Time

func main() {
	// Initialize viper first so config is available for tracing
	viper.SetDefault("MINECRAFT_CHECK_INTERVAL", "30s")
	viper.AutomaticEnv()

	// Initialize OpenTelemetry
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

	cfg, err := config.LoadWithMetadata()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	ctx := context.Background()
	dbConn, err := cfg.NewDBConnection(ctx)
	if err != nil {
		log.Fatalf("Error creating Firestore client: %v", err)
	}

	fmt.Printf("Machine agent started with check interval: %v\n", interval)

	handlePendingCommandFunc = handlePendingCommand

	// Import whitelist on startup if Firestore is empty
	if err := importWhitelistIfEmptyFunc(ctx, dbConn, cfg.InstanceName); err != nil {
		log.Printf("Error during initial whitelist import: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if err := runStatusUpdate(ctx, dbConn, cfg.InstanceName); err != nil {
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

	// Check for scheduled shutdown first (before any other processing)
	if err := checkScheduledShutdownFunc(ctx, dbConn, instanceName); err != nil {
		span.SetAttributes(attribute.String("error", "check_scheduled_shutdown_failed"))
		log.Printf("Error checking scheduled shutdown: %v", err)
		// Don't fail the entire update, just log the error
	}

	// Handle any pending commands from the controller (e.g., world save).
	if err := handlePendingCommandFunc(ctx, dbConn, instanceName); err != nil {
		span.SetAttributes(attribute.String("error", "handle_pending_command_failed"))
		log.Printf("Error handling pending command: %v", err)
	}

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

	// Sync whitelist from Firestore to Minecraft
	whitelistEnabled, err := syncWhitelistFunc(ctx, dbConn, instanceName)
	if err != nil {
		span.SetAttributes(attribute.String("error", "sync_whitelist_failed"))
		log.Printf("Error syncing whitelist: %v", err)
		// Don't fail the entire update, just log the error
	}
	span.SetAttributes(attribute.Bool("whitelist.enabled", whitelistEnabled))

	// Get current status to preserve scheduled shutdown
	currentStatus, _ := dbConn.GetStatus(ctx, instanceName)

	err = dbConn.UpdateStatus(ctx, instanceName, db.Status{
		Players:           db.Players{Current: current, Max: max},
		Timestamp:         time.Now(),
		Uptime:            uptime,
		ServerState:       db.ServerStateRunning,
		InstanceIP:        instanceIP,
		Version:           version,
		WhitelistEnabled:  whitelistEnabled,
		ScheduledShutdown: currentStatus.ScheduledShutdown,
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

// getProjectID returns the GCP project ID from instance metadata
func getProjectID() (string, error) {
	return metadata.ProjectID()
}

// getZone returns the GCP zone from instance metadata
func getZone() (string, error) {
	return metadata.Zone()
}

// getInstanceName returns the instance name from metadata
func getInstanceName() (string, error) {
	return metadata.InstanceName()
}

// stopInstance stops the current VM instance via GCP Compute API
func stopInstance(ctx context.Context) error {
	project, err := getProjectID()
	if err != nil {
		return fmt.Errorf("failed to get project ID: %w", err)
	}

	zone, err := getZone()
	if err != nil {
		return fmt.Errorf("failed to get zone: %w", err)
	}

	instance, err := getInstanceName()
	if err != nil {
		return fmt.Errorf("failed to get instance name: %w", err)
	}

	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	defer client.Close()

	log.Printf("Stopping instance %s in project %s, zone %s", instance, project, zone)

	req := &computepb.StopInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: instance,
	}

	op, err := client.Stop(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	// Wait for operation to complete (will be interrupted when VM stops)
	if err := op.Wait(ctx); err != nil {
		// This error is expected as the VM will be stopped
		log.Printf("Stop operation wait ended: %v", err)
	}

	return nil
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

// MinecraftWhitelistEntry represents an entry in Minecraft's whitelist.json
type MinecraftWhitelistEntry struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// importWhitelistIfEmpty imports whitelist.json from Minecraft if Firestore whitelist is empty
func importWhitelistIfEmpty(ctx context.Context, dbConn db.DB, instanceName string) error {
	// Check if Firestore whitelist has any entries
	entries, err := dbConn.GetWhitelistEntries(ctx, instanceName)
	if err != nil {
		log.Printf("Error checking whitelist entries: %v", err)
		// Continue with import attempt
	}

	if len(entries) > 0 {
		log.Printf("Firestore whitelist already has %d entries, skipping import", len(entries))
		return nil
	}

	log.Println("Firestore whitelist is empty, attempting to import from whitelist.json")

	// Read whitelist.json from Minecraft container
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

	// Convert and import entries to Firestore
	for _, mcEntry := range mcEntries {
		entry := db.WhitelistEntry{
			Username: mcEntry.Name,
			UUID:     mcEntry.UUID,
			AddedAt:  time.Now(),
			AddedBy:  "imported",
		}
		if err := dbConn.AddWhitelistEntry(ctx, instanceName, entry); err != nil {
			log.Printf("Error importing whitelist entry %s: %v", mcEntry.Name, err)
			continue
		}
		log.Printf("Imported whitelist entry: %s (%s)", mcEntry.Name, mcEntry.UUID)
	}

	// Also check if whitelist is enabled via server.properties or by trying whitelist list
	whitelistEnabled := getWhitelistEnabledStatus()
	if err := dbConn.SetWhitelistConfig(ctx, instanceName, db.WhitelistConfig{Enabled: whitelistEnabled}); err != nil {
		log.Printf("Error setting whitelist config: %v", err)
	}

	log.Printf("Imported %d whitelist entries from whitelist.json", len(mcEntries))
	return nil
}

// syncWhitelist syncs the whitelist from Firestore to Minecraft
// Returns whether whitelist enforcement is enabled
func syncWhitelist(ctx context.Context, dbConn db.DB, instanceName string) (bool, error) {
	// Get whitelist config from Firestore
	config, err := dbConn.GetWhitelistConfig(ctx, instanceName)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			config = db.WhitelistConfig{Enabled: false}
		} else {
			return false, fmt.Errorf("failed to get whitelist config: %w", err)
		}
	}

	// Get whitelist entries from Firestore
	firestoreEntries, err := dbConn.GetWhitelistEntries(ctx, instanceName)
	if err != nil {
		return config.Enabled, fmt.Errorf("failed to get whitelist entries: %w", err)
	}

	// Get current whitelist from Minecraft
	minecraftEntries, err := getMinecraftWhitelist()
	if err != nil {
		return config.Enabled, fmt.Errorf("failed to get minecraft whitelist: %w", err)
	}

	// Build maps for comparison
	firestoreMap := make(map[string]db.WhitelistEntry)
	for _, entry := range firestoreEntries {
		firestoreMap[entry.UUID] = entry
	}

	minecraftMap := make(map[string]bool)
	for _, entry := range minecraftEntries {
		minecraftMap[entry.UUID] = true
	}

	// Add missing players to Minecraft
	for uuid, entry := range firestoreMap {
		if !minecraftMap[uuid] {
			if err := addPlayerToMinecraftWhitelist(entry.Username); err != nil {
				log.Printf("Error adding %s to Minecraft whitelist: %v", entry.Username, err)
			} else {
				log.Printf("Added %s to Minecraft whitelist", entry.Username)
			}
		}
	}

	// Remove extra players from Minecraft
	for _, mcEntry := range minecraftEntries {
		if _, exists := firestoreMap[mcEntry.UUID]; !exists {
			if err := removePlayerFromMinecraftWhitelist(mcEntry.Name); err != nil {
				log.Printf("Error removing %s from Minecraft whitelist: %v", mcEntry.Name, err)
			} else {
				log.Printf("Removed %s from Minecraft whitelist", mcEntry.Name)
			}
		}
	}

	// Sync whitelist enabled status
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

// getMinecraftWhitelist gets the current whitelist from Minecraft via RCON
func getMinecraftWhitelist() ([]MinecraftWhitelistEntry, error) {
	// Read whitelist.json directly as RCON whitelist list doesn't give UUIDs
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

// addPlayerToMinecraftWhitelist adds a player to the Minecraft whitelist via RCON
func addPlayerToMinecraftWhitelist(username string) error {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "whitelist", "add", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcon whitelist add failed: %w, output: %s", err, string(output))
	}
	return nil
}

// removePlayerFromMinecraftWhitelist removes a player from the Minecraft whitelist via RCON
func removePlayerFromMinecraftWhitelist(username string) error {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "whitelist", "remove", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcon whitelist remove failed: %w, output: %s", err, string(output))
	}
	return nil
}

// getWhitelistEnabledStatus checks if whitelist enforcement is enabled
func getWhitelistEnabledStatus() bool {
	// Check via RCON - "whitelist list" will indicate if whitelist is on
	// If whitelist is off, players can still join without being whitelisted
	// We'll read server.properties for the authoritative value
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "cat", "/data/server.properties")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error reading server.properties: %v", err)
		return false
	}

	// Look for "white-list=true" or "enforce-whitelist=true"
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

// setWhitelistEnabled enables or disables whitelist enforcement via RCON
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

// checkScheduledShutdown checks if a shutdown is scheduled and handles warnings and execution
func checkScheduledShutdown(ctx context.Context, dbConn db.DB, instanceName string) error {
	// Get current status to check scheduled shutdown
	currentStatus, err := dbConn.GetStatus(ctx, instanceName)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// No shutdown scheduled
	if currentStatus.ScheduledShutdown == nil {
		// Reset warning state if shutdown was cancelled
		if lastScheduledShutdownTime != nil {
			log.Println("Scheduled shutdown was cancelled, resetting warning state")
			shutdownWarningState = WarningStateNone
			lastScheduledShutdownTime = nil
		}
		return nil
	}

	shutdownTime := *currentStatus.ScheduledShutdown

	// Check if this is a new/different scheduled shutdown
	if lastScheduledShutdownTime == nil || !lastScheduledShutdownTime.Equal(shutdownTime) {
		log.Printf("New scheduled shutdown detected: %v", shutdownTime)
		shutdownWarningState = WarningStateNone
		lastScheduledShutdownTime = &shutdownTime
	}

	remaining := time.Until(shutdownTime)

	// Time to shut down
	if remaining <= 0 {
		log.Println("Scheduled shutdown time reached, initiating shutdown...")
		return initiateScheduledShutdown(ctx, dbConn, instanceName)
	}

	// Send warnings based on remaining time
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

// sendMinecraftMessage sends a chat message to all players via RCON
func sendMinecraftMessage(message string) error {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "say", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcon say failed: %w, output: %s", err, string(output))
	}
	return nil
}

// saveMinecraftWorld saves the Minecraft world via RCON
func saveMinecraftWorld() error {
	cmd := execCommand("/usr/bin/docker", "exec", "minecraft.service", "rcon-cli", "save-all")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcon save-all failed: %w, output: %s", err, string(output))
	}
	log.Println("World saved successfully")
	return nil
}

// handlePendingCommand checks for and executes pending commands from the controller.
// Currently supports: "save" - triggers a world save via RCON.
func handlePendingCommand(ctx context.Context, dbConn db.DB, instanceName string) error {
	status, err := dbConn.GetStatus(ctx, instanceName)
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

	return dbConn.UpdateStatus(ctx, instanceName, status)
}

// initiateScheduledShutdown handles the shutdown process
func initiateScheduledShutdown(ctx context.Context, dbConn db.DB, instanceName string) error {
	// Send final warning
	if err := sendMinecraftMessageFunc("Server is shutting down NOW!"); err != nil {
		log.Printf("Error sending final shutdown message: %v", err)
	}

	// Save the world
	if err := saveMinecraftWorldFunc(); err != nil {
		log.Printf("Error saving world before shutdown: %v", err)
		// Continue with shutdown even if save fails
	}

	// Clear the scheduled shutdown from Firestore
	currentStatus, err := dbConn.GetStatus(ctx, instanceName)
	if err != nil {
		log.Printf("Error getting status to clear scheduled shutdown: %v", err)
	} else {
		currentStatus.ScheduledShutdown = nil
		currentStatus.Timestamp = time.Now()
		if err := dbConn.UpdateStatus(ctx, instanceName, currentStatus); err != nil {
			log.Printf("Error clearing scheduled shutdown: %v", err)
		}
	}

	// Reset local state
	shutdownWarningState = WarningStateNone
	lastScheduledShutdownTime = nil

	// Wait a moment for the save to complete
	time.Sleep(5 * time.Second)

	// Initiate VM shutdown via GCP Compute API
	log.Println("Initiating VM shutdown via GCP Compute API...")
	if err := stopInstanceFunc(ctx); err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	return nil
}
