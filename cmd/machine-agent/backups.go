package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nbyl/metio/internal/agentclient"
	"github.com/nbyl/metio/internal/backupmanifest"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const backupManifestLogPrefix = "backup-manifests: "

// processBackupManifestsFunc is a package var so tests can stub the whole
// manifest relay step.
var processBackupManifestsFunc = processBackupManifests

// processBackupManifests relays queued backup manifests to the controller's
// report API. It runs once per tick alongside the status update and follows
// an at-least-once strategy: manifests are only deleted after the controller
// acknowledged them (any 2xx), so controller outages and agent restarts never
// lose a record. Duplicate deliveries are harmless because the controller
// deduplicates reports by server and snapshot ID. Permanent rejections
// (HTTP 400) and unparsable files are quarantined by renaming so they are
// logged exactly once instead of being retried forever.
func processBackupManifests(ctx context.Context, client agentclient.AgentClient) error {
	tracer := otel.Tracer("machine-agent")
	ctx, span := tracer.Start(ctx, "processBackupManifests")
	defer span.End()

	dir := os.Getenv("MANIFEST_DIR")
	if dir == "" {
		dir = backupmanifest.DefaultDir
	}

	matches, err := filepath.Glob(filepath.Join(dir, backupmanifest.FilenamePattern))
	if err != nil {
		return fmt.Errorf("scan %s: %w", dir, err)
	}
	span.SetAttributes(attribute.Int("backups.queued", len(matches)))
	if len(matches) == 0 {
		return nil
	}
	log.Printf("%s%d manifest(s) pending", backupManifestLogPrefix, len(matches))

	version, _, _ := getMinecraftVersionFunc()
	if version == "" {
		version = "Unknown"
	}

	submitted, quarantined := 0, 0
	for _, path := range matches {
		manifest, err := backupmanifest.Load(path)
		if err != nil {
			log.Printf("%sunparsable manifest %s: %v; quarantining", backupManifestLogPrefix, filepath.Base(path), err)
			quarantineManifest(path, backupmanifest.MarkInvalid)
			quarantined++
			continue
		}

		if manifest.ServerID == "" || manifest.SnapshotID == "" || manifest.RepositoryPrefix == "" {
			log.Printf("%smanifest %s is missing required fields; quarantining", backupManifestLogPrefix, filepath.Base(path))
			quarantineManifest(path, backupmanifest.MarkRejected)
			quarantined++
			continue
		}

		report := agentclient.BackupReport{
			SnapshotID:       manifest.SnapshotID,
			RepositoryPrefix: manifest.RepositoryPrefix,
			DurationSeconds:  manifest.DurationSeconds,
			FileCount:        manifest.FileCount,
			RepositorySize:   manifest.RepositorySize,
			MinecraftVersion: version,
			Status:           manifest.Status,
		}

		err = client.SubmitBackupReport(ctx, manifest.ServerID, report)
		switch {
		case err == nil:
			if rmErr := os.Remove(path); rmErr != nil {
				// The record is already in the catalog; resubmitting on the
				// next tick is a harmless no-op thanks to deduplication.
				log.Printf("%sacknowledged %s but failed to delete it: %v",
					backupManifestLogPrefix, filepath.Base(path), rmErr)
				continue
			}
			log.Printf("%sreported snapshot %s for server %s; acknowledged",
				backupManifestLogPrefix, manifest.SnapshotID, manifest.ServerID)
			submitted++

		case isPermanentRejection(err):
			log.Printf("%scontroller rejected manifest %s: %v; quarantining",
				backupManifestLogPrefix, filepath.Base(path), err)
			quarantineManifest(path, backupmanifest.MarkRejected)
			quarantined++

		default:
			log.Printf("%ssubmitting %s failed (%v); will retry next tick",
				backupManifestLogPrefix, filepath.Base(path), err)
		}
	}

	span.SetAttributes(
		attribute.Int("backups.submitted", submitted),
		attribute.Int("backups.quarantined", quarantined),
	)
	return nil
}

// quarantineManifest renames a manifest out of the scan pattern, keeping it on
// disk for inspection. Failures are logged but otherwise ignored; a stuck file
// simply gets retried on the next tick.
func quarantineManifest(path string, mark func(string) error) {
	if err := mark(path); err != nil {
		log.Printf("%sfailed to quarantine %s: %v", backupManifestLogPrefix, path, err)
	}
}

// isPermanentRejection reports whether the controller refused the report in a
// way that retrying can never fix.
func isPermanentRejection(err error) bool {
	var statusErr *agentclient.HTTPStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusBadRequest
}
