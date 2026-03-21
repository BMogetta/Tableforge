import { WebTracerProvider } from '@opentelemetry/sdk-trace-web'
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base'
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http'
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-http'
import { OTLPLogExporter } from '@opentelemetry/exporter-logs-otlp-http'
import { MeterProvider, PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics'
import { LoggerProvider, SimpleLogRecordProcessor } from '@opentelemetry/sdk-logs'
import { resourceFromAttributes } from '@opentelemetry/resources'
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION } from '@opentelemetry/semantic-conventions'
import { W3CTraceContextPropagator } from '@opentelemetry/core'
import { SeverityNumber } from '@opentelemetry/api-logs'
import { onCLS, onFCP, onLCP, onTTFB, onINP } from 'web-vitals'
import { registerInstrumentations } from '@opentelemetry/instrumentation'
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch'

// ---------------------------------------------------------------------------
// Shared resource — identifies this service in every signal
// ---------------------------------------------------------------------------

const resource = resourceFromAttributes({
  [ATTR_SERVICE_NAME]: 'tableforge-frontend',
  [ATTR_SERVICE_VERSION]: '1.0.0',
})

// The collector is proxied through Caddy at /otlp so cookies/CORS are fine
const COLLECTOR_BASE = '/otlp'

// ---------------------------------------------------------------------------
// Traces
// ---------------------------------------------------------------------------

const traceExporter = new OTLPTraceExporter({
  url: `${COLLECTOR_BASE}/v1/traces`,
})

const tracerProvider = new WebTracerProvider({
  resource,
  spanProcessors: [new BatchSpanProcessor(traceExporter)],
})

tracerProvider.register({
  propagator: new W3CTraceContextPropagator(),
})

registerInstrumentations({
  instrumentations: [
    new FetchInstrumentation({
      propagateTraceHeaderCorsUrls: [/.*/],
      ignoreUrls: [/\/auth\//],
    }),
  ],
  tracerProvider,
})

export const tracer = tracerProvider.getTracer('tableforge-frontend')

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

const metricExporter = new OTLPMetricExporter({
  url: `${COLLECTOR_BASE}/v1/metrics`,
})

const meterProvider = new MeterProvider({
  resource,
  readers: [
    new PeriodicExportingMetricReader({
      exporter: metricExporter,
      exportIntervalMillis: 30_000,
    }),
  ],
})

const meter = meterProvider.getMeter('tableforge-frontend')

// Web Vitals histograms — collector forwards these to Prometheus
const vitalsHistogram = meter.createHistogram('web_vitals', {
  description: 'Core Web Vitals measurements',
  unit: 'ms',
})

// ---------------------------------------------------------------------------
// Logs
// ---------------------------------------------------------------------------

const logExporter = new OTLPLogExporter({
  url: `${COLLECTOR_BASE}/v1/logs`,
})

const loggerProvider = new LoggerProvider({
  resource,
  processors: [new SimpleLogRecordProcessor(logExporter)],
})

const logger = loggerProvider.getLogger('tableforge-frontend')

// ---------------------------------------------------------------------------
// Console bridge — forwards console.error and console.warn to OTLP logs
// ---------------------------------------------------------------------------
export function initConsoleBridge() {
  const originalError = console.error.bind(console)
  const originalWarn = console.warn.bind(console)

  console.error = (...args) => {
    originalError(...args)
    logger.emit({
      severityNumber: SeverityNumber.ERROR,
      severityText: 'ERROR',
      body: args.map(String).join(' '),
      attributes: { 'app.source': 'console' },
    })
  }

  console.warn = (...args) => {
    originalWarn(...args)
    logger.emit({
      severityNumber: SeverityNumber.WARN,
      severityText: 'WARN',
      body: args.map(String).join(' '),
      attributes: { 'app.source': 'console' },
    })
  }
}

// ---------------------------------------------------------------------------
// Web Vitals — push each metric as a histogram observation
// ---------------------------------------------------------------------------

function recordVital(name: string, value: number, rating: string) {
  vitalsHistogram.record(value, {
    'vital.name': name,
    'vital.rating': rating,
  })
}

export function initWebVitals() {
  onCLS(m => recordVital('CLS', m.value * 1000, m.rating))
  onFCP(m => recordVital('FCP', m.value, m.rating))
  onLCP(m => recordVital('LCP', m.value, m.rating))
  onTTFB(m => recordVital('TTFB', m.value, m.rating))
  onINP(m => recordVital('INP', m.value, m.rating)) // INP replaced FID in v4
}

// ---------------------------------------------------------------------------
// JS error handler — unhandled errors and promise rejections → OTLP logs
// ---------------------------------------------------------------------------

export function emitErrorLog(message: string, attrs: Record<string, string>) {
  logger.emit({
    severityNumber: SeverityNumber.ERROR,
    severityText: 'ERROR',
    body: message,
    attributes: {
      'app.source': 'window',
      ...attrs,
    },
  })
}

export function initErrorHandler() {
  window.addEventListener('error', event => {
    emitErrorLog(event.message, {
      'error.type': 'uncaught',
      'error.source': event.filename ?? '',
      'error.line': String(event.lineno ?? ''),
      'error.col': String(event.colno ?? ''),
      'error.stack': event.error?.stack ?? '',
    })
  })

  window.addEventListener('unhandledrejection', event => {
    const reason = event.reason
    const message =
      reason instanceof Error ? reason.message : String(reason ?? 'Unhandled promise rejection')
    emitErrorLog(message, {
      'error.type': 'unhandledrejection',
      'error.stack': reason instanceof Error ? (reason.stack ?? '') : '',
    })
  })
}

// ---------------------------------------------------------------------------
// Graceful shutdown — flush buffers before the page unloads
// ---------------------------------------------------------------------------

window.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'hidden') {
    tracerProvider.forceFlush()
    meterProvider.forceFlush()
    loggerProvider.forceFlush()
  }
})
