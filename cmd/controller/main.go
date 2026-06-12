package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/storage"
	gorillahandlers "github.com/gorilla/handlers"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/internal/config"
	"gitlab.com/nbyl/metio/internal/handlers"
	"gitlab.com/nbyl/metio/internal/pulumi"
	"gitlab.com/nbyl/metio/internal/services"
	"gitlab.com/nbyl/metio/internal/tracing"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/serviceusage/v1"
)

var Version = "dev" // default, overridden by ldflags

type servicesBundle struct {
	provisioning *services.ProvisioningService
	validation   *services.ValidationService
	setup        *services.SetupService
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

	r := handlers.New(svcs.provisioning, svcs.validation, svcs.setup)

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

	provisioningService := services.NewProvisioningService(workspaceManager, dbConn, Version)

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
	}, nil
}
