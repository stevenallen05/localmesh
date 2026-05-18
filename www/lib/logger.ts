import pino, { LoggerOptions } from 'pino';
import { trace } from '@opentelemetry/api';

// No `base:` block — identity (service_name / module_name / owned_by)
// is supplied by the Vector agent from container labels at log ingestion.
// See docs/engineering/rules/logging-platform.md §3.
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
  // pino.stdTimeFunctions.isoTime emits `"time":"<iso>"`; the contract
  // names the field `timestamp`. Custom function renames the key while
  // keeping the ms-precision ISO 8601 value.
  timestamp: () => `,"timestamp":"${new Date().toISOString()}"`,
  messageKey: 'message',
};

export const logger = pino(options);
