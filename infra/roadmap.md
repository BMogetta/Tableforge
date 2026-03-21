# Install telemetrygen
go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest

# Send test traces
telemetrygen traces --otlp-insecure --traces 10

# Send test metrics
telemetrygen metrics --otlp-insecure --metrics 10

# Observability roadmap

## Estado actual
- ✅ Trazas → OTel → Tempo
- ✅ Logs backend → OTel (bridge `log` estándar) → Loki
- ✅ Logs frontend → OTel → Loki
- ✅ Métricas backend → Prometheus nativo (`/metrics` en `:8080`)
- ✅ Métricas frontend (web vitals) → OTel → collector → Prometheus
- ✅ Grafana con dashboards: game-server, web-vitals, js-errors
- ✅ otel-collector, Tempo, Loki, Prometheus corriendo

---

## Pendiente

### 1. Promtail
⚠️ `docker_sd_configs` no funciona en WSL por permisos del socket.
- En Linux nativo funciona con la config en `./infra/promtail-config.yaml`
- Descomentar el servicio en `docker-compose.monitoring.yml`
- El filtro por container está configurado para `caddy`

### 2. Instrumentar el backend con spans
El tracer OTel está inicializado pero no hay spans en el código.
- Agregar middleware de trazas en el router de Chi (`otelchi` o manual)
- Instrumentar los handlers críticos: `handleMove`, `handleStartGame`, `handleSurrender`
- Propagar el contexto hasta los calls a store y Redis

### 3. Dashboard de trazas en Grafana
Una vez que haya spans reales en Tempo.
- Panel de latencia end-to-end por operación (frontend → backend)
- Panel de traces con errores
- Correlación logs ↔ trazas via trace ID

### 4. Alertas
- Descomentar `alertmanager` en `docker-compose.yml`
- Configurar `./infra/alertmanager/alertmanager.yml`
- Agregar reglas en Prometheus para: error rate > umbral, latencia p99 > umbral, sesiones activas caída a 0

### 5. Filtros por `job` en dashboards
Cosmético — agregar `job` label a las queries de web-vitals para evitar colisiones si el collector exporta métricas con nombres similares en el futuro.
- `web-vitals.json`: agregar `{job="otel-collector"}` a todas las queries de `otel_web_vitals_*`