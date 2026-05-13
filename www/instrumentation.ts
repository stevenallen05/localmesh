import { registerOTel } from '@vercel/otel';
import { B3Propagator } from '@opentelemetry/propagator-b3';
import { GrpcInstrumentation } from '@opentelemetry/instrumentation-grpc';

export function register() {
  registerOTel({
    serviceName: process.env.OTEL_SERVICE_NAME ?? 'www',
    propagators: [new B3Propagator()],
    instrumentations: [new GrpcInstrumentation()],
  });
}
