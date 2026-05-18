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

type WhoAmI = {
  mtls_peer_uri: string;
  user_id: string;
  user_email: string;
  trace_id: string;
};
type SayHelloReply = { message: string; who_am_i?: WhoAmI };

type GreeterClient = {
  SayHello: (
    request: { name: string },
    metadata: grpc.Metadata,
    callback: (error: unknown, response: SayHelloReply) => void,
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

  // PII-at-ingress: this is the one site in the stack where user.* /
  // enduser.* land on OTel span attributes. Downstream services receive
  // identity via gRPC metadata but must not echo PII onto their own spans
  // or indexed log labels. See lib/identity.ts header.
  const user = getUser(req);
  trace.getActiveSpan()?.setAttributes({
    'enduser.id':    user.id,
    'enduser.email': user.email,
    'user.id':       user.id,        // legacy mirror until dashboards migrate
    'user.email':    user.email,
  });
  const md = userMetadata(user);

  const target = process.env.SERVER_ADDR ?? 'server:50051';
  const client = new hello.Greeter(target, meshChannelCredentials()) as GreeterClient;

  logger.info({ name }, 'sayHello request');
  try {
    const response = await new Promise<SayHelloReply>((resolve, reject) =>
      client.SayHello({ name }, md, (err, r) => (err ? reject(err) : resolve(r))),
    );
    res.status(200).json(response);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'unknown';
    logger.error({ err: message }, 'sayHello failed');
    res.status(500).json({ error: 'gRPC call failed', message });
  }
}
