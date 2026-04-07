# Observability

The full observability stack starts with `make up-all` (requires `--profile monitoring`).

## Stack

```
Services (Go + Frontend)
    |
    v
OTel Collector (gRPC :4317, HTTP :4318)
    |
    +---> Tempo     (distributed traces)
    +---> Loki      (logs)
    +---> Prometheus (metrics)
    |
    v
Grafana (dashboards, queries, alerts)
```

## Components

### OpenTelemetry Collector

Receives traces, metrics, and logs from all services via gRPC (port 4317) and HTTP (port 4318). Also receives browser traces from the frontend via Traefik (`/otlp/*`, priority 200).

Config: `infra/collector-config.yaml`

### Tempo

Distributed tracing backend. Stores full request traces with spans.

- Config: `infra/tempo-config.yaml`
- Volume: `tempo-data:/var/tempo`

### Loki

Log aggregation with LogQL queries. No indexing -- label-based.

- Config: `infra/loki-config.yaml`
- Volume: `loki-data:/loki`

### Prometheus

Metrics scraper and time-series database. Scrapes `/metrics` endpoints from services. 15-day retention. Also receives OpenTelemetry metrics via remote write.

- Config: `infra/prometheus.yaml`
- Volume: `prometheus-data:/prometheus`

### Grafana

Dashboards and visualization. Pre-provisioned with datasources (Tempo, Loki, Prometheus) and dashboards.

- Datasources: `infra/grafana-datasources.yaml`
- Dashboards: `infra/grafana/provisioning/dashboards/`
- Access: `http://${GRAFANA_HOST}` (default: `grafana.localhost`)
- Volume: `grafana-data:/var/lib/grafana`

## Service Integration

All Go services follow the same telemetry pattern:

```go
serviceName := config.Env("OTEL_SERVICE_NAME", "service-name")
otlpEndpoint := config.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
shutdownTelemetry, err := telemetry.Setup(ctx, serviceName, otlpEndpoint)
defer shutdownTelemetry(shutdownCtx)
```

The OTel SDK instruments the chi router, gRPC, and database queries automatically. When the monitoring profile is not up, the SDK retries silently -- no crashes.

The frontend sends browser traces via the OpenTelemetry JS SDK to `/otlp/v1/traces` (routed by Traefik to the collector). Web Vitals are also collected.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `OTEL_SERVICE_NAME` | per service | Service identifier in traces |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTel collector address |
| `GRAFANA_HOST` | `grafana.localhost` | Grafana domain for Traefik routing |
| `GRAFANA_ADMIN_PASSWORD` | -- | Grafana admin password |
| `GRAFANA_MCP_TOKEN` | -- | Service account token for Claude Code integration |
