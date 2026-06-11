package services

import (
	"context"
	"fmt"
	"log"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"gitlab.com/nbyl/metio/internal/db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// stopInstanceFn and startInstanceFn are function variables for testability.
var stopInstanceFn = func(ctx context.Context, req *computepb.StopInstanceRequest) error {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	_, err = client.Stop(ctx, req)
	return err
}

var startInstanceFn = func(ctx context.Context, req *computepb.StartInstanceRequest) error {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	_, err = client.Start(ctx, req)
	return err
}

func StopInstance(ctx context.Context, project, zone, instance string) error {
	req := &computepb.StopInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: instance,
	}
	log.Printf("Stopping instance %s in %s/%s", instance, project, zone)
	if err := stopInstanceFn(ctx, req); err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}
	log.Printf("Instance %s stopped successfully", instance)
	return nil
}

func StartInstance(ctx context.Context, project, zone, instance string) error {
	req := &computepb.StartInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: instance,
	}
	log.Printf("Starting instance %s in %s/%s", instance, project, zone)
	if err := startInstanceFn(ctx, req); err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}
	log.Printf("Instance %s started successfully", instance)
	return nil
}

// BackupCoordinator manages the command-and-ack pattern for world saves via the machine-agent.
type BackupCoordinator struct {
	dbConn db.DB
}

func NewBackupCoordinator(dbConn db.DB) *BackupCoordinator {
	return &BackupCoordinator{dbConn: dbConn}
}

func (b *BackupCoordinator) TriggerWorldSave(ctx context.Context, instanceName string) error {
	statusEntry, err := b.dbConn.GetStatus(ctx, instanceName)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Printf("No status document for instance %s — was never started, skipping backup", instanceName)
			return nil
		}
		return fmt.Errorf("failed to get server status for backup: %w", err)
	}
	statusEntry.PendingCommand = "save"
	statusEntry.PendingCommandResult = ""
	if err := b.dbConn.UpdateStatus(ctx, instanceName, statusEntry); err != nil {
		return fmt.Errorf("failed to write backup command: %w", err)
	}
	log.Printf("Triggered world save for instance %s", instanceName)
	return nil
}

func (b *BackupCoordinator) WaitForCommandAck(ctx context.Context, instanceName string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		statusEntry, err := b.dbConn.GetStatus(ctx, instanceName)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				log.Printf("Status document vanished for instance %s — was never started, skipping backup ack", instanceName)
				return "", nil
			}
			return "", fmt.Errorf("failed to poll status for ack: %w", err)
		}

		if statusEntry.PendingCommandResult != "" {
			if statusEntry.PendingCommandResult == "completed" {
				return "completed", nil
			}
			return statusEntry.PendingCommandResult, fmt.Errorf("command result: %s", statusEntry.PendingCommandResult)
		}

		time.Sleep(pollInterval)
	}

	return "", fmt.Errorf("timeout waiting for command ack after %v", timeout)
}

// WaitForServerHealthy polls the server status until the machine-agent reports RUNNING or the timeout elapses.
func WaitForServerHealthy(ctx context.Context, dbConn db.DB, instanceName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	pollInterval := 5 * time.Second

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		status, err := dbConn.GetStatus(ctx, instanceName)
		if err != nil {
			log.Printf("Error polling server status for %s: %v", instanceName, err)
			time.Sleep(pollInterval)
			continue
		}

		if status.ServerState.IsRunning() {
			log.Printf("Server %s is healthy (state: RUNNING)", instanceName)
			return nil
		}

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("timeout waiting for server %s to become healthy after %v", instanceName, timeout)
}
