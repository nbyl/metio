package tracing

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
)

var tp *sdktrace.TracerProvider

func InitTracer() error {
	return InitTracerWithDetails("metio-service", "1.0.0")
}

func InitTracerWithDetails(serviceName, serviceVersion string) error {
	// Detect GCP environment and get project ID
	projectID := os.Getenv("GCP_PROJECT")
	if projectID == "" {
		log.Printf("Warning: GCP_PROJECT not set, using default detection")
	}

	// Get deployment environment
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		log.Printf("Warning: ENVIRONMENT not set, using default 'development'")
		environment = "development"
	}

	// Retrieve and store Google application-default credentials
	creds, err := oauth.NewApplicationDefault(context.Background())
	if err != nil {
		panic(err)
	}

	res, err := resource.New(context.Background(),
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			attribute.String("gcp.project_id", projectID),
			attribute.String("deployment.environment", environment),
		),
	)
	if err != nil {
		return err
	}

	// Configure OTLP exporter for Google Cloud Trace with proper headers
	headers := map[string]string{
		"X-Goog-User-Project": projectID,
	}

	exporter, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithEndpoint("telemetry.googleapis.com"),
		otlptracegrpc.WithHeaders(headers),
		otlptracegrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)),
	)
	if err != nil {
		log.Printf("Failed to create Cloud Trace exporter: %v", err)
		return err
	}

	// Create tracer provider with sampling for production
	tp = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	log.Printf("OpenTelemetry tracer initialized for service: %s (sending to Google Cloud Trace, project: %s)", serviceName, projectID)
	return nil
}

func ShutdownTracer() {
	if tp != nil {
		tp.Shutdown(context.Background())
	}
}
