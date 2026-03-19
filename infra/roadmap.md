# Observability Roadmap

## Stack Overview

```
                        ┌─────────────────────────────────────────────┐
  All traffic ────────► │               Caddy (single entry point)    │
  localhost/            │   /           → Frontend                    │
                        │   /api/*      → Backend                     │
                        │   /metrics/*  → (optional internal routes)  │
                        └──────────────┬──────────────────────────────┘
                                       │
                     ┌─────────────────┼─────────────────┐
                     ▼                 ▼                  ▼
                  Frontend          Backend          (Observability UIs)
                                  │        │
                             PostgreSQL   Redis
```

Every request flows through Caddy first. This makes it the most valuable single
point for observability: if Caddy metrics are healthy, traffic is reaching the
system. If Caddy shows errors or latency spikes before your backend does, the
problem is at the proxy level.

### Full observability stack

| Component | Role |
|-----------|------|
| **OpenTelemetry Collector** | Receives all signals (metrics, logs, traces) from all services and routes them |
| **Prometheus** | Stores metrics; scraped by OTel or directly |
| **Loki** | Stores logs |
| **Jaeger** | Stores and visualizes distributed traces |
| **Grafana** | Unified visualization layer — queries all three backends |

---

## Phase 1 — Verify your setup is actually working (1–2 days)

Before building anything, confirm data is flowing end to end.

### Check each datasource in Grafana

1. Open Grafana → **Explore** (compass icon in the sidebar)
2. Select **Prometheus** → run the query `up` → you should see a `1` for each
   scrape target that is alive
3. Select **Loki** → query `{container="your-backend-name"}` → you should see
   log lines
4. Select **Jaeger** (or your traces datasource) → search for recent traces →
   you should see request spans

If any of these return nothing, the problem is in the pipeline before Grafana,
not in Grafana itself. Check the OTel Collector logs first.

### Verify Caddy is exporting metrics

Caddy exposes a Prometheus-compatible metrics endpoint. Make sure your
`Caddyfile` has the admin API enabled and your Prometheus config is scraping it:

```caddy
{
  admin localhost:2019
}
```

```yaml
# prometheus.yml (scrape config)
scrape_configs:
  - job_name: caddy
    static_configs:
      - targets: ["caddy:2019"]
    metrics_path: /metrics
```

Key Caddy metrics to confirm are present:
- `caddy_http_requests_total`
- `caddy_http_request_duration_seconds`
- `caddy_http_response_size_bytes`

---

## Phase 2 — The metrics that actually matter (3–5 days)

### The RED method — for every HTTP service

Apply this to Caddy first (since all traffic passes through it), then to your
backend separately.

| Signal | What it measures | Example query (PromQL) |
|--------|-----------------|------------------------|
| **Rate** | Requests per second | `rate(caddy_http_requests_total[1m])` |
| **Errors** | % of requests returning 5xx | `rate(caddy_http_requests_total{code=~"5.."}[1m]) / rate(caddy_http_requests_total[1m])` |
| **Duration** | Request latency (p99) | `histogram_quantile(0.99, rate(caddy_http_request_duration_seconds_bucket[5m]))` |

> **Why Caddy first?** Because it sees 100% of traffic. Your backend might only
> see traffic for specific routes. Caddy's RED metrics are the ground truth for
> the whole system.

### The USE method — for system resources

| Signal | What it measures |
|--------|-----------------|
| **Utilization** | CPU %, memory % in use |
| **Saturation** | Work queued waiting to be processed |
| **Errors** | System-level errors (OOM kills, disk errors) |

These come from `node_exporter` (host-level) or cAdvisor (container-level).
Key metrics: `process_cpu_seconds_total`, `container_memory_usage_bytes`.

### PostgreSQL-specific metrics

If you have `postgres_exporter` running:

| Metric | What to watch |
|--------|--------------|
| `pg_stat_activity_count` | Active connections — if this saturates your `max_connections`, queries start failing |
| `pg_stat_database_blks_hit / (blks_hit + blks_read)` | Cache hit ratio — should be above 95% |
| `pg_stat_statements_mean_exec_time` | Average query execution time |
| `pg_locks_count` | Lock contention — spikes here indicate transaction bottlenecks |

### Redis-specific metrics

| Metric | What to watch |
|--------|--------------|
| `redis_keyspace_hits_total / (hits + misses)` | Cache hit ratio — low values mean the cache isn't helping |
| `redis_memory_used_bytes` | Watch against `maxmemory` — evictions degrade performance silently |
| `redis_commands_processed_total` | Commands per second |
| `redis_connected_clients` | Connection count |

---

## Phase 3 — Your first dashboard (2–3 days)

### Recommended layout

```
┌──────────────────────────────────────────────────────────────┐
│                    CADDY (entry point)                       │
│  Requests/sec  │  Error rate %  │  Latency p99  │  Active    │
├──────────────────────────────────────────────────────────────┤
│                 BACKEND SERVICE                              │
│  Requests/sec  │  Error rate %  │  Latency p99               │
├──────────────────────────────────────────────────────────────┤
│             LATENCY BREAKDOWN BY ROUTE                       │
│  heatmap or bar chart grouped by Caddy route matcher         │
├───────────────────────────┬──────────────────────────────────┤
│  CPU % (backend)          │  Memory (backend)                │
├───────────────────────────┼──────────────────────────────────┤
│  PostgreSQL connections   │  Redis hit ratio                 │
└───────────────────────────┴──────────────────────────────────┘
```

### Import pre-built dashboards

Rather than building from scratch, import these in **Dashboards → Import**:

| Dashboard ID | What it covers |
|-------------|----------------|
| `1860` | Node Exporter Full (host-level CPU, memory, disk, network) |
| `9628` | PostgreSQL Database |
| `763` | Redis Dashboard for Prometheus |
| `14876` | Caddy reverse proxy metrics |

Then build one custom dashboard on top that combines the RED metrics from Caddy
and your backend in a single view. That single dashboard is what you watch
during a stress test.

---

## Phase 4 — Alerts (2–3 days)

Alerts are what make metrics useful when you are not actively watching. Set
these up before running stress tests — otherwise you only learn about failures
after the fact.

| Alert | Condition | Why it matters |
|-------|-----------|----------------|
| High error rate (Caddy) | `> 1%` for 2 min | Requests are failing before they reach the backend |
| High error rate (backend) | `> 5%` for 2 min | Application-level failures |
| High latency | p99 `> 2s` for 5 min | Degraded user experience |
| CPU saturation | `> 85%` for 5 min | Risk of becoming a bottleneck |
| Memory pressure | `> 90%` | Risk of OOM kill |
| DB connection pool near limit | `> 80% of max_connections` | Queries will start queuing |
| Redis memory near limit | `> 85% of maxmemory` | Evictions will start degrading cache |

In Grafana, alerts can be created directly from any panel: **Edit panel → Alert
tab**. You can route them to email, Slack, PagerDuty, or a webhook.

---

## Phase 5 — Stress testing with observability (the actual goal)

### Recommended tools

- **[k6](https://k6.io/)** — scripted load tests in JavaScript, integrates with
  Prometheus natively via the k6 Prometheus remote write extension
- **[hey](https://github.com/rakyll/hey)** — simpler, no setup, good for quick
  spike tests: `hey -n 10000 -c 100 http://localhost/api/endpoint`
- **[Grafana k6](https://grafana.com/docs/k6/latest/)** — if you want test
  results directly in your Grafana instance

### Stress test workflow

1. Open your combined dashboard on a second screen before starting
2. Run the load generator against `localhost` — Caddy handles routing
3. Watch the dashboard in real time:
   - Latency rises → identifies the saturation point
   - Error rate appears → identifies the breaking point
   - CPU/memory trends → identifies resource-bound vs I/O-bound issues
4. When you see a latency spike, switch to **Jaeger** and search for traces
   from that time window — Jaeger will show you which specific function or
   database call caused the slowdown
5. Cross-reference with **Loki**: in Grafana Explore, select a time range
   around the spike and filter logs for errors

### What to look for

| Observation | Likely cause |
|-------------|-------------|
| Caddy latency spikes but backend latency is normal | Caddy is the bottleneck (unlikely for a hobby project) |
| Backend latency spikes, DB connection count rises sharply | PostgreSQL connection pool exhaustion — consider pgBouncer or increasing pool size |
| Backend latency spikes, Redis hit ratio drops | Cache misses under load — cache stampede or evictions |
| Memory grows during load and doesn't recover after | Memory leak — compare heap before and after |
| Error rate spikes at a specific request rate and then recovers | You have found the system's concurrency limit |

### Since all traffic goes through Caddy

Caddy lets you add per-route metrics by labeling routes in your Caddyfile. This
means you can break down latency and error rate by route in Prometheus:

```caddy
localhost {
  handle /api/* {
    reverse_proxy backend:8080
  }
  handle /* {
    reverse_proxy frontend:3000
  }
}
```

Caddy will attach the matched route to the metric labels automatically, so you
can query:

```promql
# Latency for API routes only
histogram_quantile(0.99,
  rate(caddy_http_request_duration_seconds_bucket{handler="reverse_proxy"}[5m])
)
```

---

## Phase 6 — Custom business metrics (optional but valuable)

Once infrastructure metrics are in place, you can add domain-specific metrics
directly from your application code using the OTel SDK. These flow through the
same OTel Collector → Prometheus → Grafana pipeline automatically.

```python
# Python example
from opentelemetry import metrics
meter = metrics.get_meter("my-app")

checkout_counter = meter.create_counter(
    "orders_processed_total",
    description="Total number of processed orders"
)
checkout_counter.add(1, {"status": "success", "payment_method": "card"})

db_query_histogram = meter.create_histogram(
    "db_query_duration_seconds",
    description="Duration of database queries"
)
```

```typescript
// TypeScript/Node.js example
import { metrics } from '@opentelemetry/api';
const meter = metrics.getMeter('my-app');

const requestCounter = meter.createCounter('api_requests_total');
requestCounter.add(1, { route: '/api/users', method: 'GET' });
```

These custom metrics become available in Prometheus like any other metric and
can be added to your dashboard to correlate technical load with application
behavior.

---

## Priority order if you want to start today

1. **Verify the pipeline** — run `up` in Grafana Explore against Prometheus; check Loki has logs
2. **Import pre-built dashboards** — IDs listed above, zero PromQL required
3. **Add Caddy metrics** — confirm `caddy_http_requests_total` is in Prometheus
4. **Build the RED panel for Caddy** — this alone covers 80% of what you need to know
5. **Set the 5 basic alerts** — error rate, latency, CPU, memory, DB connections
6. **Run a first stress test with `hey`** — watch the dashboard, then dig into Jaeger for the bottleneck

The gap between `docker compose logs` and a real dashboard is large, but the
path to get there is well-defined. The goal is not many metrics — it is the
right metrics with alerts, so the system tells you when something is wrong
instead of you having to go look.