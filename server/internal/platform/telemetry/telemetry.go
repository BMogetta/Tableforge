// Package telemetry initializes OpenTelemetry tracing and metrics.
// Call Setup() once at startup and defer the returned shutdown function.
// If the collector is unreachable, Setup logs a warning and returns a no-op
// shutdown so the server always starts.
package telemetry

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// noop is returned when telemetry setup fails so the server keeps running.
func noop(_ context.Context) error { return nil }

// Setup initializes OTel tracing and metrics exporters.
// It returns a shutdown function that must be deferred by the caller.
// If setup fails for any reason the server continues without telemetry.
func Setup(ctx context.Context, serviceName, otlpEndpoint string) (func(context.Context) error, error) {
	conn, err := grpc.Dial( //nolint:staticcheck
		otlpEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("telemetry: could not connect to collector at %s, continuing without telemetry: %v", otlpEndpoint, err)
		return noop, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		log.Printf("telemetry: could not create resource, continuing without telemetry: %v", err)
		return noop, nil
	}

	// Traces
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return noop, fmt.Errorf("telemetry: trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Metrics
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return noop, fmt.Errorf("telemetry: metric exporter: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	log.Printf("telemetry: connected to collector at %s", otlpEndpoint)

	return func(ctx context.Context) error {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			return err
		}
		return meterProvider.Shutdown(ctx)
	}, nil
}
