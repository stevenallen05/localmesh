import type { NextApiRequest, NextApiResponse } from 'next';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import path from 'path';
import { trace } from '@opentelemetry/api';
import { logger } from '../../lib/logger';
import { meshChannelCredentials } from '../../lib/grpc-credentials';
import { getUser, userMetadata } from '../../lib/identity';

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

type PostgresStatsReply = {
  num_backends: number;
  xact_commit: string;          // int64 marshalled as string (longs: String)
  xact_rollback: string;
  cache_hit_ratio: number;
  db_size_bytes: string;
};

type GreeterClient = {
  PrintPostgresStats: (
    request: Record<string, never>,
    metadata: grpc.Metadata,
    callback: (error: unknown, response: PostgresStatsReply) => void,
  ) => void;
  close: () => void;
};

export default async function handler(req: NextApiRequest, res: NextApiResponse) {
  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  // PII-at-ingress: see lib/identity.ts header.
  const user = getUser(req);
  trace.getActiveSpan()?.setAttributes({
    'enduser.id':    user.id,
    'enduser.email': user.email,
    'user.id':       user.id,
    'user.email':    user.email,
  });
  const md = userMetadata(user);

  const target = process.env.SERVER_ADDR ?? 'server:50051';
  const client = new hello.Greeter(target, meshChannelCredentials()) as GreeterClient;

  try {
    const stats = await new Promise<PostgresStatsReply>((resolve, reject) =>
      client.PrintPostgresStats({}, md, (err, r) => (err ? reject(err) : resolve(r))),
    );
    logger.info({ stats }, 'postgres stats');
    res.status(200).json(stats);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'unknown';
    logger.error({ err: message }, 'print-postgres-stats failed');
    res.status(500).json({ error: 'gRPC call failed', message });
  } finally {
    client.close();
  }
}
