package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// backupUnitFilePath is the systemd unit rendered onto every VM by the
	// provisioning cloud-config. It carries the restic repository, password
	// and container image, so the agent can source them even on VMs that were
	// provisioned before restore support existed.
	backupUnitFilePath = "/etc/systemd/system/minecraft-backup.service"

	stagingDirName   = ".restore-staging"
	recoveryDirName  = "recovery"
	restoreContainer = "metio-restore"
)

// minecraftBaseDir is the mount point of the Minecraft data disk.
var minecraftBaseDir = "/mnt/disks/minecraft"

type backupUnitConfig struct {
	Repository string
	Password   string
	Image      string
}

// parseBackupUnitConfig extracts the restic environment and the container image
// from a rendered minecraft-backup.service systemd unit.
func parseBackupUnitConfig(content string) (*backupUnitConfig, error) {
	cfg := &backupUnitConfig{}

	envRe := regexp.MustCompile(`-e (RESTIC_REPOSITORY|RESTIC_PASSWORD)=(\S+)`)
	for _, m := range envRe.FindAllStringSubmatch(content, -1) {
		switch m[1] {
		case "RESTIC_REPOSITORY":
			cfg.Repository = m[2]
		case "RESTIC_PASSWORD":
			cfg.Password = m[2]
		}
	}
	if cfg.Repository == "" || cfg.Password == "" {
		return nil, fmt.Errorf("%s is missing RESTIC_REPOSITORY or RESTIC_PASSWORD", backupUnitFilePath)
	}

	image, err := extractBackupImage(content)
	if err != nil {
		return nil, err
	}
	cfg.Image = image
	return cfg, nil
}

// extractBackupImage returns the image reference of the backup container: the
// final indented line of the ExecStart block, which cloud-config renders as the
// bare image name. Non-indented lines are top-level systemd directives and end
// the block.
func extractBackupImage(content string) (string, error) {
	var lastLine string
	inExecStart := false
	for _, raw := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if !isContinuationLine(raw) {
			inExecStart = strings.HasPrefix(trimmed, "ExecStart=")
			if inExecStart {
				lastLine = ""
			}
			continue
		}
		if inExecStart {
			lastLine = trimmed
		}
	}

	fields := strings.Fields(strings.TrimSuffix(lastLine, "\\"))
	if len(fields) == 1 && !strings.HasPrefix(fields[0], "-") {
		return fields[0], nil
	}
	return "", fmt.Errorf("could not determine backup image from %s", backupUnitFilePath)
}

// isContinuationLine reports whether a raw line from a systemd unit is an
// indented continuation of a previous directive.
func isContinuationLine(raw string) bool {
	return strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t")
}

func stopMinecraftServices() error {
	cmd := execCommand("systemctl", "stop", "minecraft.service", "minecraft-backup.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl stop failed: %w, output: %s", err, string(output))
	}
	return nil
}

func startMinecraftServices() error {
	cmd := execCommand("systemctl", "start", "minecraft.service", "minecraft-backup.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl start failed: %w, output: %s", err, string(output))
	}
	return nil
}

// runResticRestore restores the snapshot into a staging directory next to the
// live world directory using a one-off container built from the same backup
// image (which bundles the restic binary and GCS support).
func runResticRestore(snapshotID string, cfg *backupUnitConfig) error {
	cmd := execCommand("/usr/bin/docker", "run", "--rm", "--name", restoreContainer,
		"--network", "host",
		"-v", minecraftBaseDir+":/mnt",
		"-e", "RESTIC_REPOSITORY="+cfg.Repository,
		"-e", "RESTIC_PASSWORD="+cfg.Password,
		cfg.Image,
		"restic", "restore", snapshotID, "--target", "/mnt/"+stagingDirName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restic restore failed: %w, output: %s", err, string(output))
	}
	return nil
}

// performRestore swaps the restored staging directory into place while keeping
// the previous world in a recovery directory. On any failure the previous world
// stays in place (or is rolled back into place) and is never deleted.
func performRestore(baseDir, snapshotID string, cfg *backupUnitConfig) error {
	dataDir := filepath.Join(baseDir, "data")
	stagingDir := filepath.Join(baseDir, stagingDirName)
	recoveryDir := filepath.Join(baseDir, recoveryDirName, fmt.Sprintf("%d", time.Now().Unix()))

	// Best-effort cleanup of leftovers from a previously failed attempt.
	os.RemoveAll(stagingDir)

	if err := runResticRestore(snapshotID, cfg); err != nil {
		os.RemoveAll(stagingDir)
		return err
	}
	defer os.RemoveAll(stagingDir)

	if err := os.MkdirAll(filepath.Dir(recoveryDir), 0o755); err != nil {
		return fmt.Errorf("failed to create recovery directory: %w", err)
	}

	if err := os.Rename(dataDir, recoveryDir); err != nil {
		return fmt.Errorf("failed to move current world to recovery directory %s: %w", recoveryDir, err)
	}

	if err := os.Rename(stagingDir, dataDir); err != nil {
		if rbErr := os.Rename(recoveryDir, dataDir); rbErr != nil {
			return fmt.Errorf("activating restored world failed (%v) and rollback failed (%v); "+
				"previous world remains at %s", err, rbErr, recoveryDir)
		}
		return fmt.Errorf("activating restored world failed, rolled back to previous world: %w", err)
	}

	log.Printf("Restore of snapshot %s complete. Previous world preserved at %s", snapshotID, recoveryDir)
	return nil
}

// restoreMinecraftWorld stops the Minecraft containers, restores the given
// restic snapshot over the world directory and restarts the containers. On
// failure the previous world is kept (or restored) and an error describing the
// rollback outcome is returned.
func restoreMinecraftWorld(snapshotID string) error {
	content, err := osReadFile(backupUnitFilePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", backupUnitFilePath, err)
	}
	cfg, err := parseBackupUnitConfig(string(content))
	if err != nil {
		return err
	}

	log.Printf("Stopping Minecraft services for restore...")
	if err := stopMinecraftServices(); err != nil {
		return fmt.Errorf("failed to stop Minecraft services: %w", err)
	}

	if err := performRestore(minecraftBaseDir, snapshotID, cfg); err != nil {
		log.Printf("Restore failed: %v", err)
		if startErr := startMinecraftServices(); startErr != nil {
			return fmt.Errorf("%v (additionally, restarting Minecraft services failed: %v)", err, startErr)
		}
		log.Printf("Previous world is back in place and Minecraft was restarted")
		return err
	}

	if err := startMinecraftServices(); err != nil {
		return fmt.Errorf("restore succeeded but restarting Minecraft services failed: %w", err)
	}
	return nil
}

var restoreMinecraftWorldFunc = restoreMinecraftWorld
