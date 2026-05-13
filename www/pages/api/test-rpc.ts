import type { NextApiRequest, NextApiResponse } from 'next';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import { metrics } from '@opentelemetry/api';
import { promisify } from 'util';
import path from 'path';

// Module-scope so the meter/histogram are created once per process, not per request.
const meter = metrics.getMeter('www');
const testRpcDuration = meter.createHistogram('test_rpc_duration_seconds', {
  description: 'test-rpc API route handler duration in seconds',
});

// WORKDIR /app in the container; proto/ lands at /app/proto/.
const PROTO_PATH = path.resolve(process.cwd(), 'proto/hello.proto');

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});
const { hello } = grpc.loadPackageDefinition(packageDefinition) as any;

type GreeterClient = {
  SayHello: (
    request: { name: string },
    callback: (error: unknown, response: { message: string }) => void,
  ) => void;
};

export default async function handler(req: NextApiRequest, res: NextApiResponse) {
  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' });
  }
  const { name } = req.body ?? {};
  if (typeof name !== 'string') {
    return res.status(400).json({ error: 'name (string) is required' });
  }

  const target = process.env.SERVER_ADDR ?? 'server:50051';
  const client = new hello.Greeter(
    target,
    grpc.credentials.createInsecure(),
  ) as GreeterClient;
  const sayHello = promisify(client.SayHello.bind(client));

  const start = Date.now();
  try {
    const response = await sayHello({ name });
    res.status(200).json(response);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'unknown';
    res.status(500).json({ error: 'gRPC call failed', message });
  } finally {
    testRpcDuration.record((Date.now() - start) / 1000);
  }
}
