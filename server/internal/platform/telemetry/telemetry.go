package telemetry

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func noop(_ context.Context) error { return nil }

func Setup(ctx context.Context, serviceName, otlpEndpoint string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		log.Printf("telemetry: could not create resource, continuing without telemetry: %v", err)
		return noop, nil
	}

	// ── Traces ────────────────────────────────────────────────────────────────
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Printf("telemetry: could not connect to collector at %s, continuing without telemetry: %v", otlpEndpoint, err)
		return noop, nil
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// ── Metrics ───────────────────────────────────────────────────────────────
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(otlpEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return noop, fmt.Errorf("telemetry: metric exporter: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	// ── Logs ──────────────────────────────────────────────────────────────────
	logExporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(otlpEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return noop, fmt.Errorf("telemetry: log exporter: %w", err)
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(loggerProvider)

	log.Printf("telemetry: connected to collector at %s", otlpEndpoint)

	InitLogBridge(serviceName)

	return func(ctx context.Context) error {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			return err
		}
		if err := meterProvider.Shutdown(ctx); err != nil {
			return err
		}
		return loggerProvider.Shutdown(ctx)
	}, nil
}
