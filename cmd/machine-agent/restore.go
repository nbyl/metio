package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	// minecraftUnit, backupUnit and agentUnit name the systemd units (and,
	// since the units run their containers with --name %n, the docker
	// containers) for the Minecraft server, its backup service, and the
	// machine-agent itself.
	minecraftUnit = "minecraft.service"
	backupUnit    = "minecraft-backup.service"
	agentUnit     = "metio-machine-agent.service"

	// worldMountDest is the mount destination inside the backup container that
	// maps to the host's live world directory on the data disk.
	worldMountDest = "/data"

	// hostSystemctlPath is the absolute path to systemctl on the host. It is
	// reached via nsenter, which switches into the host mount namespace.
	hostSystemctlPath = "/usr/bin/systemctl"
)

// dockerBin is the docker CLI used by the agent. It is a variable so tests can
// stub it via the exec seams.
var dockerBin = "/usr/bin/docker"

// hostSystemctl runs systemctl on the host by delegating to a short-lived
// privileged helper container (the agent's own image, which ships nsenter and
// util-linux) that enters the host PID, mount, UTS and network namespaces.
//
// The regular agent container runs as a non-root user with only the docker
// socket, and a non-root uid has no effective CAP_SYS_ADMIN, so it cannot
// nsenter itself even when --privileged. Instead it uses the docker socket —
// which already authorizes creating privileged containers — to spin up this
// helper.
func hostSystemctl(ctx context.Context, agentImage string, args ...string) ([]byte, error) {
	cmdArgs := []string{
		"run", "--rm",
		"--privileged",
		"--pid=host",
		"--user", "0",
		"--entrypoint", "/usr/bin/nsenter",
		agentImage,
		"-t", "1", "-m", "-u", "-i", "-n",
		hostSystemctlPath,
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := execCommandContext(ctx, dockerBin, cmdArgs...)
	return cmd.CombinedOutput()
}

// stopMinecraftServices stops the Minecraft server and backup systemd units on
// the host. Both units run with Restart=always, so stopping the docker
// containers directly would be reverted by systemd; they must be stopped
// through the host systemd, which the helper reaches via nsenter.
func stopMinecraftServices(ctx context.Context, agentImage string) error {
	output, err := hostSystemctl(ctx, agentImage, "stop", minecraftUnit, backupUnit)
	if err != nil {
		return fmt.Errorf("systemctl stop failed: %w, output: %s", err, string(output))
	}
	return nil
}

// startMinecraftServices starts the Minecraft server and backup systemd units
// on the host.
func startMinecraftServices(ctx context.Context, agentImage string) error {
	output, err := hostSystemctl(ctx, agentImage, "start", minecraftUnit, backupUnit)
	if err != nil {
		return fmt.Errorf("systemctl start failed: %w, output: %s", err, string(output))
	}
	return nil
}

// containerImage returns the image reference for the running container with
// the given name. This is used to discover the backup image (the restore run
// must use a fresh container after the unit — and its --rm container — is
// stopped) and the agent's own image (the nsenter provider for the helper).
func containerImage(ctx context.Context, name string) (string, error) {
	cmd := execCommandContext(ctx, dockerBin, "inspect", "--format", "{{.Config.Image}}", name)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker inspect %s image failed: %w, output: %s", name, err, string(output))
	}
	image := strings.TrimSpace(string(output))
	if image == "" {
		return "", fmt.Errorf("container %s has no image reference", name)
	}
	return image, nil
}

// containerMountSource returns the host source path for the mount of the given
// container whose destination is dest. It is used to discover the live world
// directory from the backup container's /data mount.
func containerMountSource(ctx context.Context, name, dest string) (string, error) {
	cmd := execCommandContext(ctx, dockerBin, "inspect", "--format",
		"{{ range .Mounts }}{{ if eq .Destination \""+dest+"\" }}{{ .Source }}{{ end }}{{ end }}", name)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker inspect %s mounts failed: %w, output: %s", name, err, string(output))
	}
	source := strings.TrimSpace(string(output))
	if source == "" {
		return "", fmt.Errorf("container %s has no mount at destination %s", name, dest)
	}
	return source, nil
}

// runResticRestore restores the given snapshot over the live world directory by
// running a fresh, one-shot container from the backup image. The backup unit's
// container is --rm, so it no longer exists once its unit is stopped; restic
// must run in its own container mounting the discovered world directory.
//
// --network host exposes the GCP metadata server so restic can use the VM's
// service-account credentials against the GCS-backed repository.
//
// The snapshot is rooted at the backup container's /data tree, so the restore
// target is prefixed with ":data" and --target is set to the world mount point:
// snapshotID:/data lands the snapshot's files directly in the world directory.
//
// --delete is deliberately omitted. It would trip restic's safety guard when
// combined with a root target, and applied without --exclude it would delete
// every file absent from the snapshot — including the *.jar server binaries and
// logs that the backup cycle's EXCLUDES skips. The trade-off is that files
// created after the snapshot remain instead of being pruned.
func runResticRestore(ctx context.Context, backupImage, worldDir, snapshotID, repository, password string) error {
	cmdArgs := []string{
		"run", "--rm",
		"--network", "host",
		"-e", "RESTIC_REPOSITORY=" + repository,
		"-e", "RESTIC_PASSWORD=" + password,
		"-v", worldDir + ":" + worldMountDest,
		"--entrypoint", "/usr/bin/restic",
		backupImage,
		"restore", snapshotID + ":" + worldMountDest, "--target", worldMountDest,
	}

	cmd := execCommandContext(ctx, dockerBin, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restic restore failed: %w, output: %s", err, string(output))
	}
	return nil
}

// restoreMinecraftWorld stops the Minecraft services, restores the given restic
// snapshot in place over the world directory, and restarts the services. On
// failure the services are still restarted so the server comes back up with the
// previous world in place.
func restoreMinecraftWorld(snapshotID, repository, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), restoreTimeout)
	defer cancel()

	agentImage, err := containerImage(ctx, agentUnit)
	if err != nil {
		return fmt.Errorf("unable to identify agent image: %w", err)
	}
	backupImage, err := containerImage(ctx, backupUnit)
	if err != nil {
		return fmt.Errorf("unable to identify backup image: %w", err)
	}
	worldDir, err := containerMountSource(ctx, backupUnit, worldMountDest)
	if err != nil {
		return fmt.Errorf("unable to identify world directory: %w", err)
	}

	log.Printf("Stopping Minecraft services for restore...")
	if err := stopMinecraftServices(ctx, agentImage); err != nil {
		return fmt.Errorf("failed to stop Minecraft services: %w", err)
	}

	if err := runResticRestore(ctx, backupImage, worldDir, snapshotID, repository, password); err != nil {
		log.Printf("Restore failed: %v", err)
		if startErr := startMinecraftServices(ctx, agentImage); startErr != nil {
			return fmt.Errorf("%v (additionally, restarting Minecraft services failed: %v)", err, startErr)
		}
		log.Printf("Previous world is back in place and Minecraft was restarted")
		return err
	}

	if err := startMinecraftServices(ctx, agentImage); err != nil {
		return fmt.Errorf("restore succeeded but restarting Minecraft services failed: %w", err)
	}
	log.Printf("Restore of snapshot %s complete.", snapshotID)
	return nil
}

var restoreMinecraftWorldFunc = restoreMinecraftWorld

// restoreTimeout bounds the whole restore operation (service stop, restic run,
// service start).
var restoreTimeout = 30 * time.Minute
