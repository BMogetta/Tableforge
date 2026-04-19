package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"

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

// Setup initialises traces, metrics and logs exporters pointed at the OTel
// collector. It sets slog.Default() to a handler that forwards to the OTel
// log pipeline with correct severity levels.
//
// If the collector is unreachable, Setup logs a warning and returns a noop
// shutdown — the service continues running without telemetry.
//
// Call the returned shutdown function in a deferred block in main().
func Setup(ctx context.Context, serviceName, otlpEndpoint string) (func(context.Context) error, error) {
	// Text handler to stdout is always active. When the OTel pipeline comes
	// up below we fan-out to both — the OTel handler alone would swallow
	// logs whenever the collector is down, which masks real errors on the
	// os.Exit paths (config.MustEnv, shared/redis.Connect).
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(textHandler))

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		slog.Warn("telemetry: could not create resource, continuing without telemetry", "error", err)
		return noop, nil
	}

	// ── Traces ────────────────────────────────────────────────────────────────
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		slog.Warn("telemetry: could not connect to collector, continuing without telemetry",
			"endpoint", otlpEndpoint,
			"error", err,
		)
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

	// Fan-out to stdout AND the OTel pipeline. Levels flow correctly to Loki
	// via OTel; stdout stays as the ground-truth source for kubectl logs and
	// as a safety net when the collector is unreachable.
	slog.SetDefault(slog.New(NewMultiHandler(textHandler, NewOtelHandler(serviceName))))

	slog.Info("telemetry: connected", "endpoint", otlpEndpoint)

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

func noop(_ context.Context) error { return nil }
