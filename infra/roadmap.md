# Install telemetrygen
go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest

# Send test traces
telemetrygen traces --otlp-insecure --traces 10

# Send test metrics
telemetrygen metrics --otlp-insecure --metrics 10