import { registerOTel } from '@vercel/otel';
import { GrpcInstrumentation } from '@opentelemetry/instrumentation-grpc';

export function register() {
  registerOTel({
    serviceName: process.env.OTEL_SERVICE_NAME ?? 'www',
    instrumentations: [new GrpcInstrumentation()],
  });
}
