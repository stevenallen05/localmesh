import type { NextConfig } from 'next';

const config: NextConfig = {
  // Keep these out of the bundler so @opentelemetry/instrumentation-grpc can
  // monkey-patch @grpc/grpc-js at require time — otherwise the patched module
  // and the one the API route imports diverge, and W3C traceparent headers
  // never make it onto the wire.
  serverExternalPackages: [
    '@grpc/grpc-js',
    '@grpc/proto-loader',
    '@opentelemetry/instrumentation-grpc',
  ],
};

export default config;
