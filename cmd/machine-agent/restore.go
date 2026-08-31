package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

const (
	// minecraftServiceContainer and backupContainer name the docker containers
	// that systemd runs for the Minecraft server and its backup service. The
	// container name equals the systemd unit name (docker runs them with
	// --name %n).
	minecraftServiceContainer = "minecraft.service"
	backupContainer           = "minecraft-backup.service"
)

// hostCommand is the prefix used to run a command on the host from inside the
// (privileged, --pid=host) machine-agent container via nsenter.
var hostCommand = []string{"/usr/bin/nsenter", "-t", "1", "-m", "-u", "-i", "-n"}

// stopMinecraftServices stops the Minecraft server and backup systemd units on
// the host. Both units run with Restart=always, so stopping the docker
// containers directly would be reverted by systemd; they must be stopped
// through the host systemd, which the agent reaches via nsenter.
func stopMinecraftServices() error {
	output, err := execHost("systemctl", "stop", minecraftServiceContainer, backupContainer)
	if err != nil {
		return fmt.Errorf("systemctl stop failed: %w, output: %s", err, string(output))
	}
	return nil
}

// startMinecraftServices starts the Minecraft server and backup systemd units
// on the host.
func startMinecraftServices() error {
	output, err := execHost("systemctl", "start", minecraftServiceContainer, backupContainer)
	if err != nil {
		return fmt.Errorf("systemctl start failed: %w, output: %s", err, string(output))
	}
	return nil
}

// runResticRestore restores the given snapshot into the live world directory
// (/data inside the backup container, which maps to the data disk) by
// exec-ing into the already-running minecraft-backup container. The source
// repository and password are supplied explicitly so the restore does not
// depend on the target server's own backup repository configuration.
func runResticRestore(snapshotID, repository, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), restoreTimeout)
	defer cancel()

	cmd := execCommandContext(ctx, "/usr/bin/docker", "exec",
		"-e", "RESTIC_REPOSITORY="+repository,
		"-e", "RESTIC_PASSWORD="+password,
		backupContainer,
		"restic", "restore", snapshotID, "--target", "/data")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restic restore failed: %w, output: %s", err, string(output))
	}
	return nil
}

// execHost runs a command on the host via nsenter into the host PID namespace.
func execHost(args ...string) ([]byte, error) {
	fullArgs := append(append([]string{}, hostCommand...), args...)
	cmd := execCommand(fullArgs[0], fullArgs[1:]...)
	return cmd.CombinedOutput()
}

// restoreMinecraftWorld stops the Minecraft services, restores the given restic
// snapshot in place over the world directory (running restic inside the
// minecraft-backup container), and restarts the services. On failure the
// services are still restarted so the server comes back up with the previous
// world in place.
func restoreMinecraftWorld(snapshotID, repository, password string) error {
	log.Printf("Stopping Minecraft services for restore...")
	if err := stopMinecraftServices(); err != nil {
		return fmt.Errorf("failed to stop Minecraft services: %w", err)
	}

	if err := runResticRestore(snapshotID, repository, password); err != nil {
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
	log.Printf("Restore of snapshot %s complete.", snapshotID)
	return nil
}

var restoreMinecraftWorldFunc = restoreMinecraftWorld

// restoreTimeout bounds a single restic restore command.
var restoreTimeout = 30 * time.Minute
