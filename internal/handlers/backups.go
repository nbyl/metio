package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/nbyl/metio/internal/services"
	"go.opentelemetry.io/otel"
)

var ErrBackupCleanupUnavailable = errors.New("backup cleanup unavailable in this deployment mode")

type BackupCleanupSweeper interface {
	RunSweep(ctx context.Context) (*services.CleanupResult, error)
}

var newBackupCleanupSweeper = func(ctx context.Context) (BackupCleanupSweeper, error) {
	dbConn, cfg, err := getDBConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	if cfg.OperationMode != "cloudtasks" {
		return nil, ErrBackupCleanupUnavailable
	}
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	bucketName := fmt.Sprintf("%s-%s-backups", cfg.ProjectID, cfg.Environment)
	return services.NewBackupCleanupService(dbConn, services.NewStorageAdapter(storageClient), bucketName), nil
}

func HandleBackupCleanup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("backups-handler")
	ctx, span := tracer.Start(ctx, "handleBackupCleanup")
	defer span.End()

	if !verifyPubSubAuth(r) {
		log.Print("Unauthorized backup cleanup request")
		WriteJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sweeper, err := newBackupCleanupSweeper(ctx)
	if errors.Is(err, ErrBackupCleanupUnavailable) {
		http.Error(w, "backup cleanup is not available in this deployment mode", http.StatusNotImplemented)
		return
	}
	if err != nil {
		log.Printf("Failed to initialize backup cleanup service: %v", err)
		WriteJSONError(w, "failed to initialize cleanup service", http.StatusInternalServerError)
		return
	}

	result, err := sweeper.RunSweep(ctx)
	if err != nil {
		log.Printf("Backup cleanup sweep failed: %v", err)
		WriteJSONError(w, "cleanup sweep failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
