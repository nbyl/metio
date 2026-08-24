package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nbyl/metio/internal/db"
)

type CleanupResult struct {
	ServersScanned int   `json:"servers_scanned"`
	ServersCleaned int   `json:"servers_cleaned"`
	ServersFailed  int   `json:"servers_failed"`
	ObjectsDeleted int64 `json:"objects_deleted"`
}

type BackupCleanupService struct {
	db         db.DB
	storage    StorageClient
	bucketName string
}

func NewBackupCleanupService(dbConn db.DB, storage StorageClient, bucketName string) *BackupCleanupService {
	return &BackupCleanupService{
		db:         dbConn,
		storage:    storage,
		bucketName: bucketName,
	}
}

func (s *BackupCleanupService) RunSweep(ctx context.Context) (*CleanupResult, error) {
	backups, err := s.db.ListBackups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups for cleanup sweep: %w", err)
	}

	now := time.Now()

	type serverDeletion struct {
		allExpired bool
	}
	servers := make(map[string]*serverDeletion)
	for _, backup := range backups {
		if backup == nil || backup.ServerID == "" {
			continue
		}
		if backup.ServerDeletedAt == nil || backup.RetentionUntil == nil {
			continue
		}
		state, ok := servers[backup.ServerID]
		if !ok {
			state = &serverDeletion{allExpired: true}
			servers[backup.ServerID] = state
		}
		if backup.RetentionUntil.After(now) {
			state.allExpired = false
		}
	}

	result := &CleanupResult{ServersScanned: len(servers)}

	bucket := s.storage.Bucket(s.bucketName)
	for serverID, state := range servers {
		if !state.allExpired {
			continue
		}

		prefix := fmt.Sprintf("servers/%s/restic/", serverID)
		deleted, err := bucket.DeletePrefix(ctx, prefix)
		if err != nil {
			log.Printf("Backup cleanup: failed to delete prefix %s in bucket %s; will retry next sweep: %v", prefix, s.bucketName, err)
			result.ServersFailed++
			continue
		}
		result.ObjectsDeleted += deleted

		if err := s.db.DeleteServerBackups(ctx, serverID); err != nil {
			log.Printf("Backup cleanup: repository prefix %s deleted but catalog removal failed for server %s; will retry next sweep: %v", prefix, serverID, err)
			result.ServersFailed++
			continue
		}

		result.ServersCleaned++
		log.Printf("Backup cleanup: removed expired backups of server %s (%d objects)", serverID, deleted)
	}

	return result, nil
}
