import { SeverityNumber } from '@opentelemetry/api-logs'
import type { Tracer } from '@opentelemetry/api'

// ---------------------------------------------------------------------------
// In test mode, disable all telemetry to avoid 405 spam from missing OTel
// collector and to keep test output clean.
// ---------------------------------------------------------------------------

const TELEMETRY_DISABLED =
  import.meta.env.VITE_TEST_MODE === 'true' || import.meta.env.DEV

// ---------------------------------------------------------------------------
// No-op stubs used when telemetry is disabled
// ---------------------------------------------------------------------------

const noopTracer = {
  startSpan: () => ({
    end: () => {},
    setStatus: () => {},
    setAttribute: () => {},
    recordException: () => {},
    setAttributes: () => {},
    addEvent: () => {},
    isRecording: () => false,
    spanContext: () => ({ traceId: '', spanId: '', traceFlags: 0 }),
    updateName: () => {},
  }),
  startActiveSpan: (...args: unknown[]) => {
    const fn = args[args.length - 1] as (span: ReturnType<Tracer['startSpan']>) => unknown
    return fn(noopTracer.startSpan('') as ReturnType<Tracer['startSpan']>)
  },
} as unknown as Tracer

// Exported tracer — real or no-op
export let tracer: Tracer = noopTracer

export let emitErrorLog = (_message: string, _attrs: Record<string, string>) => {}
export let initWebVitals = () => {}
export let initErrorHandler = () => {}
export let initConsoleBridge = () => {}

// ---------------------------------------------------------------------------
// Real telemetry — only loaded when not in test/dev mode
// ---------------------------------------------------------------------------

if (!TELEMETRY_DISABLED) {
  const { WebTracerProvider } = await import('@opentelemetry/sdk-trace-web')
  const { BatchSpanProcessor } = await import('@opentelemetry/sdk-trace-base')
  const { OTLPTraceExporter } = await import('@opentelemetry/exporter-trace-otlp-http')
  const { OTLPMetricExporter } = await import('@opentelemetry/exporter-metrics-otlp-http')
  const { OTLPLogExporter } = await import('@opentelemetry/exporter-logs-otlp-http')
  const { MeterProvider, PeriodicExportingMetricReader } = await import('@opentelemetry/sdk-metrics')
  const { LoggerProvider, SimpleLogRecordProcessor } = await import('@opentelemetry/sdk-logs')
  const { resourceFromAttributes } = await import('@opentelemetry/resources')
  const { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION } = await import('@opentelemetry/semantic-conventions')
  const { W3CTraceContextPropagator } = await import('@opentelemetry/core')
  const { registerInstrumentations } = await import('@opentelemetry/instrumentation')
  const { FetchInstrumentation } = await import('@opentelemetry/instrumentation-fetch')
  const { onCLS, onFCP, onLCP, onTTFB, onINP } = await import('web-vitals')

  const resource = resourceFromAttributes({
    [ATTR_SERVICE_NAME]: 'recess-frontend',
    [ATTR_SERVICE_VERSION]: '1.0.0',
  })

  const COLLECTOR_BASE = '/otlp'

  // --- Traces ---

  const traceExporter = new OTLPTraceExporter({ url: `${COLLECTOR_BASE}/v1/traces` })
  const tracerProvider = new WebTracerProvider({
    resource,
    spanProcessors: [new BatchSpanProcessor(traceExporter)],
  })
  tracerProvider.register({ propagator: new W3CTraceContextPropagator() })
  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/.*/],
        ignoreUrls: [/\/auth\//, /\/otlp\//],
      }),
    ],
    tracerProvider,
  })
  tracer = tracerProvider.getTracer('recess-frontend')

  // --- Metrics ---

  const metricExporter = new OTLPMetricExporter({ url: `${COLLECTOR_BASE}/v1/metrics` })
  const meterProvider = new MeterProvider({
    resource,
    readers: [new PeriodicExportingMetricReader({ exporter: metricExporter, exportIntervalMillis: 30_000 })],
  })
  const vitalsHistogram = meterProvider.getMeter('recess-frontend').createHistogram('web_vitals', {
    description: 'Core Web Vitals measurements',
    unit: 'ms',
  })

  // --- Logs ---

  const logExporter = new OTLPLogExporter({ url: `${COLLECTOR_BASE}/v1/logs` })
  const loggerProvider = new LoggerProvider({
    resource,
    processors: [new SimpleLogRecordProcessor(logExporter)],
  })
  const logger = loggerProvider.getLogger('recess-frontend')

  // --- Override exported functions with real implementations ---

  emitErrorLog = (message: string, attrs: Record<string, string>) => {
    logger.emit({
      severityNumber: SeverityNumber.ERROR,
      severityText: 'ERROR',
      body: message,
      attributes: { 'app.source': 'window', ...attrs },
    })
  }

  initConsoleBridge = () => {
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

  initWebVitals = () => {
    const recordVital = (name: string, value: number, rating: string) => {
      vitalsHistogram.record(value, { 'vital.name': name, 'vital.rating': rating })
    }
    onCLS(m => recordVital('CLS', m.value * 1000, m.rating))
    onFCP(m => recordVital('FCP', m.value, m.rating))
    onLCP(m => recordVital('LCP', m.value, m.rating))
    onTTFB(m => recordVital('TTFB', m.value, m.rating))
    onINP(m => recordVital('INP', m.value, m.rating))
  }

  initErrorHandler = () => {
    window.addEventListener('error', async event => {
      const { getDeviceContextAttrs } = await import('./device')
      emitErrorLog(event.message, {
        'error.type': 'uncaught',
        'error.source': event.filename ?? '',
        'error.line': String(event.lineno ?? ''),
        'error.col': String(event.colno ?? ''),
        'error.stack': event.error?.stack ?? '',
        ...getDeviceContextAttrs(),
      })
    })

    window.addEventListener('unhandledrejection', async event => {
      const { getDeviceContextAttrs } = await import('./device')
      const reason = event.reason
      const message =
        reason instanceof Error ? reason.message : String(reason ?? 'Unhandled promise rejection')
      emitErrorLog(message, {
        'error.type': 'unhandledrejection',
        'error.stack': reason instanceof Error ? (reason.stack ?? '') : '',
        ...getDeviceContextAttrs(),
      })
    })
  }

  // --- Graceful shutdown ---
  window.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') {
      tracerProvider.forceFlush()
      meterProvider.forceFlush()
      loggerProvider.forceFlush()
    }
  })
}
