package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/storage"
	gorillahandlers "github.com/gorilla/handlers"
	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/handlers"
	"github.com/nbyl/metio/internal/pulumi"
	"github.com/nbyl/metio/internal/services"
	"github.com/nbyl/metio/internal/tracing"
	"github.com/spf13/viper"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/serviceusage/v1"
)

var Version = "dev" // default, overridden by ldflags

type servicesBundle struct {
	provisioning *services.ProvisioningService
	validation   *services.ValidationService
	setup        *services.SetupService
	config       config.Config
}

func main() {
	// Initialize viper first so config is available for tracing
	viper.AutomaticEnv()
	viper.SetDefault("PORT", "8080")

	// Initialize OpenTelemetry
	if err := tracing.InitTracerWithDetails("metio-controller", Version); err != nil {
		log.Printf("Failed to initialize tracer: %v", err)
	}
	if err := tracing.InitMetrics(); err != nil {
		log.Printf("Failed to initialize metrics: %v", err)
	}
	defer tracing.ShutdownTracer()

	// Initialize services
	svcs, err := initServices()
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

	r := handlers.New(svcs.provisioning, svcs.validation, svcs.setup, svcs.provisioning, &svcs.config)

	// Wrap router with CORS middleware (only enabled in dev mode)
	handler := handlers.CORSMiddleware(r)

	port := viper.GetString("PORT")
	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), gorillahandlers.LoggingHandler(os.Stdout, handler)))
}

func initServices() (*servicesBundle, error) {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	cfg.ControllerVersion = Version

	dbConn, err := cfg.NewDBConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create db connection: %w", err)
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	setupSvc := services.NewSetupService(cfg, dbConn, services.NewStorageAdapter(storageClient))
	stateBucket, err := setupSvc.EnsureStateBucket(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Pulumi state bucket: %w", err)
	}

	workspaceManager, err := pulumi.NewWorkspaceManager(ctx, cfg.ProjectID, stateBucket)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace manager: %w", err)
	}

	var executor services.OperationExecutor
	if cfg.OperationMode == "cloudtasks" {
		ctClient, err := cloudtasks.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create cloud tasks client: %w", err)
		}
		executor = services.NewCloudTasksExecutor(
			ctClient,
			cfg.ProjectID,
			cfg.CloudTasksRegion,
			cfg.CloudTasksQueue,
			cfg.BaseURL,
			cfg.ControllerServiceAccount,
			dbConn,
			30*time.Minute,
		)
		log.Printf("Using Cloud Tasks executor (queue: %s/%s)", cfg.CloudTasksRegion, cfg.CloudTasksQueue)
	} else {
		executor = services.NewGoroutineExecutor(30 * time.Minute)
		log.Print("Using goroutine executor")
	}

	provisioningService := services.NewProvisioningService(workspaceManager, dbConn, Version, executor, cfg.BackupDeletedServerRetentionDays)
	provisioningService.SetSaveAckTimeout(cfg.SaveAckTimeout)
	provisioningService.SetBackupRestoreConfig(fmt.Sprintf("%s-%s-backups", cfg.ProjectID, cfg.Environment), cfg.BackupResticPassword)

	suSvc, err := serviceusage.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create serviceusage client: %w", err)
	}

	rmSvc, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource manager client: %w", err)
	}

	validationService := services.NewValidationService(cfg,
		services.NewServiceUsageAdapter(suSvc),
		services.NewResourceManagerAdapter(rmSvc))

	return &servicesBundle{
		provisioning: provisioningService,
		validation:   validationService,
		setup:        setupSvc,
		config:       cfg,
	}, nil
}
