package tracing

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

var tp *sdktrace.TracerProvider

func InitTracer() error {
	return InitTracerWithDetails("metio-service", "1.0.0")
}

func InitTracerWithDetails(serviceName, serviceVersion string) error {
	// Detect GCP environment and get project ID
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		log.Printf("Warning: GOOGLE_CLOUD_PROJECT not set, using default detection")
	}

	res, err := resource.New(context.Background(),
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return err
	}

	// Configure OTLP exporter for Google Cloud Trace with proper headers
	headers := map[string]string{
		"X-Goog-User-Project": projectID,
	}

	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint("https://cloudtrace.googleapis.com:443"),
		otlptracehttp.WithHeaders(headers),
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
