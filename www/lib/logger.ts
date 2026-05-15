import pino, { LoggerOptions } from 'pino';
import { trace } from '@opentelemetry/api';

const SERVICE = process.env.OTEL_SERVICE_NAME ?? 'www';

function parseResourceAttrs(): Record<string, string> {
  const raw = process.env.OTEL_RESOURCE_ATTRIBUTES ?? '';
  const out: Record<string, string> = {};
  for (const pair of raw.split(',')) {
    const [k, v] = pair.split('=').map((s) => s.trim());
    if (k && v !== undefined) out[k] = v;
  }
  return out;
}
const RESOURCE = parseResourceAttrs();

const options: LoggerOptions = {
  formatters: {
    level: (label) => ({ level: label }),
    log: (obj) => {
      const ctx = trace.getActiveSpan()?.spanContext();
      return ctx?.traceId
        ? { ...obj, trace_id: ctx.traceId, span_id: ctx.spanId }
        : obj;
    },
  },
  base: {
    service: SERVICE,
    module_name: RESOURCE.module_name ?? '',
    owned_by: RESOURCE.owned_by ?? '',
  },
  // pino.stdTimeFunctions.isoTime emits `"time":"<iso>"`; the contract
  // (spec §4.5) names the field `timestamp`. Custom function renames the
  // key while keeping the millisecond-precision ISO 8601 value.
  timestamp: () => `,"timestamp":"${new Date().toISOString()}"`,
  messageKey: 'message',
};

export const logger = pino(options);
