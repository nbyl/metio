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
	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/handlers"
	"gitlab.com/nbyl/metio/pulumi"
	"gitlab.com/nbyl/metio/services"
	"gitlab.com/nbyl/metio/tracing"
)

var Version = "dev" // default, overridden by ldflags

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

	// Initialize provisioning service as a singleton
	provisioningService, err := initProvisioningService()
	if err != nil {
		log.Printf("Warning: provisioning service not available: %v", err)
	}

	r := handlers.New(provisioningService)

	// Wrap router with CORS middleware (only enabled in dev mode)
	handler := handlers.CORSMiddleware(r)

	port := viper.GetString("PORT")
	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), gorillahandlers.LoggingHandler(os.Stdout, handler)))
}

func initProvisioningService() (*services.ProvisioningService, error) {
	ctx := context.Background()
	cfg := config.Load()

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

	return services.NewProvisioningService(workspaceManager, dbConn), nil
}
