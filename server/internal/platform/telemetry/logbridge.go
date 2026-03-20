// logbridge.go
package telemetry

import (
	"context"
	"io"
	"log"
	"strings"

	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// InitLogBridge redirects the standard logger output to the OTel log pipeline.
// Call this after Setup() so the logger provider is already registered.
// All log.Printf / log.Println calls will be forwarded to Loki via the collector.
func InitLogBridge(serviceName string) {
	log.SetOutput(&otelWriter{
		logger: global.Logger(serviceName),
	})
	// Keep date/time prefix so logs are still readable locally
	log.SetFlags(log.LstdFlags)
}

// otelWriter implements io.Writer and forwards each line to the OTel log pipeline.
type otelWriter struct {
	logger otellog.Logger
}

func (w *otelWriter) Write(p []byte) (n int, err error) {
	body := strings.TrimRight(string(p), "\n")

	var record otellog.Record
	record.SetSeverity(otellog.SeverityInfo)
	record.SetSeverityText("INFO")
	record.SetBody(otellog.StringValue(body))

	w.logger.Emit(context.Background(), record)

	return len(p), nil
}

// discardWriter silences the standard logger output when telemetry is disabled.
// Without this, logs would disappear entirely if InitLogBridge is never called.
var _ io.Writer = (*otelWriter)(nil)
