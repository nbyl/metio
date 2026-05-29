package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/db"
	"google.golang.org/api/googleapi"
)

var (
	ErrBucketExistsWithoutLabels = errors.New("bucket exists but lacks Metio labels — delete it or set PULUMI_STATE_BUCKET explicitly")
	ErrInsufficientPermissions   = errors.New("insufficient permissions to create or access the state bucket")
	ErrBillingNotEnabled         = errors.New("GCP billing is not enabled for this project")
	ErrQuotaExceeded             = errors.New("GCP quota exceeded for bucket creation")
)

type SetupService struct {
	projectID   string
	region      string
	environment string
	db          db.DB
	storage     StorageClient
}

func NewSetupService(cfg config.Config, dbConn db.DB, sc StorageClient) *SetupService {
	return &SetupService{
		projectID:   cfg.ProjectID,
		region:      cfg.Region,
		environment: cfg.Environment,
		db:          dbConn,
		storage:     sc,
	}
}

func (s *SetupService) EnsureStateBucket(ctx context.Context) (string, error) {
	if bucket := os.Getenv("PULUMI_STATE_BUCKET"); bucket != "" {
		log.Printf("using state bucket from env PULUMI_STATE_BUCKET: %s", bucket)
		return bucket, nil
	}

	settings, err := s.db.GetPulumiSettings(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read Pulumi settings: %w", err)
	}

	if settings != nil && settings.StateBucket != "" {
		if err := s.verifyBucketLabels(ctx, settings.StateBucket); err != nil {
			return "", err
		}
		log.Printf("using existing state bucket from settings: %s", settings.StateBucket)
		return settings.StateBucket, nil
	}

	bucketName := s.defaultBucketName()

	if err := s.verifyBucketLabels(ctx, bucketName); err != nil {
		if !errors.Is(err, ErrBucketExistsWithoutLabels) {
			return "", err
		}
		return "", fmt.Errorf("bucket %q exists without Metio labels: %w", bucketName, ErrBucketExistsWithoutLabels)
	}

	if err := s.createBucketIfNotExists(ctx, bucketName); err != nil {
		return "", err
	}

	now := time.Now()
	pulumiSettings := &db.PulumiSettings{
		StateBucket:   bucketName,
		Initialized:   true,
		InitializedAt: now,
		InitializedBy: "controller",
	}
	if err := s.db.SetPulumiSettings(ctx, pulumiSettings); err != nil {
		return "", fmt.Errorf("failed to persist Pulumi settings: %w", err)
	}

	log.Printf("state bucket ready: %s", bucketName)
	return bucketName, nil
}

func (s *SetupService) defaultBucketName() string {
	return fmt.Sprintf("%s-metio-pulumi-state", s.environment)
}

func (s *SetupService) verifyBucketLabels(ctx context.Context, name string) error {
	attrs, err := s.storage.Bucket(name).Attrs(ctx)
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return s.classifyGCPError(err)
	}

	if attrs.Labels == nil {
		return ErrBucketExistsWithoutLabels
	}
	if attrs.Labels["managed-by"] != "metio" || attrs.Labels["purpose"] != "pulumi-state" {
		return ErrBucketExistsWithoutLabels
	}
	return nil
}

func (s *SetupService) createBucketIfNotExists(ctx context.Context, name string) error {
	_, err := s.storage.Bucket(name).Attrs(ctx)
	if err == nil {
		log.Printf("bucket %s already exists with valid labels, adopting", name)
		return nil
	}
	if !isNotFoundError(err) {
		return s.classifyGCPError(err)
	}

	attrs := &storage.BucketAttrs{
		Location:                 s.region,
		StorageClass:             "STANDARD",
		VersioningEnabled:        true,
		PublicAccessPrevention:   storage.PublicAccessPreventionEnforced,
		Labels: map[string]string{
			"managed-by": "metio",
			"purpose":    "pulumi-state",
		},
	}

	if err := s.storage.Bucket(name).Create(ctx, s.projectID, attrs); err != nil {
		return s.classifyGCPError(err)
	}

	log.Printf("created state bucket %s in %s", name, s.region)
	return nil
}

func (s *SetupService) classifyGCPError(err error) error {
	if err == nil {
		return nil
	}

	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		switch gerr.Code {
		case 403:
			if strings.Contains(strings.ToLower(gerr.Message), "billing") {
				return fmt.Errorf("%w: %v", ErrBillingNotEnabled, gerr.Message)
			}
			return fmt.Errorf("%w: %v", ErrInsufficientPermissions, gerr.Message)
		case 409:
			return nil
		case 429:
			return fmt.Errorf("%w: %v", ErrQuotaExceeded, gerr.Message)
		}
	}

	return err
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) && gerr.Code == 404 {
		return true
	}
	return strings.Contains(err.Error(), "doesn't exist") ||
		strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "No such object")
}
