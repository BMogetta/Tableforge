package telemetry

import (
	"context"
	"log/slog"

	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// otelHandler implements slog.Handler and forwards records to the OTel log pipeline.
// This replaces the old log.Printf bridge — slog levels map correctly to OTel severities.
type otelHandler struct {
	logger otellog.Logger
	attrs  []slog.Attr
	group  string
}

func NewOtelHandler(serviceName string) slog.Handler {
	return &otelHandler{
		logger: global.Logger(serviceName),
	}
}

func (h *otelHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

func (h *otelHandler) Handle(ctx context.Context, r slog.Record) error {
	var record otellog.Record

	record.SetSeverity(slogLevelToOtel(r.Level))
	record.SetSeverityText(r.Level.String())
	record.SetBody(otellog.StringValue(r.Message))
	record.SetTimestamp(r.Time)

	// Forward slog attributes as OTel log attributes.
	r.Attrs(func(a slog.Attr) bool {
		record.AddAttributes(otellog.String(a.Key, a.Value.String()))
		return true
	})
	for _, a := range h.attrs {
		record.AddAttributes(otellog.String(a.Key, a.Value.String()))
	}

	h.logger.Emit(ctx, record)
	return nil
}

func (h *otelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &otelHandler{logger: h.logger, attrs: newAttrs, group: h.group}
}

func (h *otelHandler) WithGroup(name string) slog.Handler {
	return &otelHandler{logger: h.logger, attrs: h.attrs, group: name}
}

func slogLevelToOtel(level slog.Level) otellog.Severity {
	switch {
	case level >= slog.LevelError:
		return otellog.SeverityError
	case level >= slog.LevelWarn:
		return otellog.SeverityWarn
	case level >= slog.LevelInfo:
		return otellog.SeverityInfo
	default:
		return otellog.SeverityDebug
	}
}
