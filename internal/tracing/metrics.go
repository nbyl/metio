package tracing

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"gitlab.com/nbyl/metio/internal/config"
)

var (
	requestCounter         metric.Int64Counter
	responseDuration       metric.Float64Histogram
	errorCounter           metric.Int64Counter
	dbOperationCounter     metric.Int64Counter
	eventProcessingCounter metric.Int64Counter
	statusUpdateCounter    metric.Int64Counter
	environmentAttr        attribute.KeyValue
)

func InitMetrics() error {
	meter := otel.Meter("metio")

	// Initialize environment attribute from config
	cfg := config.Load()
	environmentAttr = attribute.String("deployment.environment", cfg.Environment)

	var err error
	requestCounter, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return err
	}

	responseDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
	)
	if err != nil {
		return err
	}

	errorCounter, err = meter.Int64Counter(
		"errors_total",
		metric.WithDescription("Total number of errors"),
	)
	if err != nil {
		return err
	}

	dbOperationCounter, err = meter.Int64Counter(
		"db_operations_total",
		metric.WithDescription("Total number of database operations"),
	)
	if err != nil {
		return err
	}

	eventProcessingCounter, err = meter.Int64Counter(
		"events_processed_total",
		metric.WithDescription("Total number of Pub/Sub events processed"),
	)
	if err != nil {
		return err
	}

	statusUpdateCounter, err = meter.Int64Counter(
		"status_updates_total",
		metric.WithDescription("Total number of status updates from machine-agent"),
	)
	if err != nil {
		return err
	}

	log.Printf("OpenTelemetry metrics initialized")
	return nil
}

// Helper functions for recording metrics
func RecordRequest(method, path string) {
	if requestCounter != nil {
		requestCounter.Add(context.Background(), 1,
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.path", path),
				environmentAttr,
			),
		)
	}
}

func RecordError(operation string) {
	if errorCounter != nil {
		errorCounter.Add(context.Background(), 1,
			metric.WithAttributes(
				attribute.String("error.operation", operation),
				environmentAttr,
			),
		)
	}
}

func RecordDBOperation(operation string) {
	if dbOperationCounter != nil {
		dbOperationCounter.Add(context.Background(), 1,
			metric.WithAttributes(
				attribute.String("db.operation", operation),
				environmentAttr,
			),
		)
	}
}

func RecordEventProcessed(eventType string) {
	if eventProcessingCounter != nil {
		eventProcessingCounter.Add(context.Background(), 1,
			metric.WithAttributes(
				attribute.String("event.type", eventType),
				environmentAttr,
			),
		)
	}
}

func RecordStatusUpdate(instanceName, state string) {
	if statusUpdateCounter != nil {
		statusUpdateCounter.Add(context.Background(), 1,
			metric.WithAttributes(
				attribute.String("instance.name", instanceName),
				attribute.String("instance.state", state),
				environmentAttr,
			),
		)
	}
}
