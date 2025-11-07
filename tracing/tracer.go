package tracing

import (
	"context"
	"log"

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
	// Detect GCP environment
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

	// Configure OTLP exporter (Google Cloud Trace)
	exporter, err := otlptracehttp.New(context.Background())
	if err != nil {
		return err
	}

	// Create tracer provider
	tp = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	log.Printf("OpenTelemetry tracer initialized for service: %s", serviceName)
	return nil
}

func ShutdownTracer() {
	if tp != nil {
		tp.Shutdown(context.Background())
	}
}
