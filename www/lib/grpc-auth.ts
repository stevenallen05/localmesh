// Tiny helper: copy the inbound Authorization header into gRPC metadata
// so the server-side auth_interceptor can validate the JWT.
//
// www is identity-free post-T9 — no parsing, no validation, no fallback.
// The Authorization header is set by oauth2-proxy at the Caddy ingress
// (via --pass-authorization-header); it lands here as an opaque string
// and gets forwarded as gRPC `authorization` metadata.

import type { NextApiRequest } from 'next';
import type { IncomingMessage } from 'http';
import * as grpc from '@grpc/grpc-js';

export function forwardAuth(req: NextApiRequest | IncomingMessage): grpc.Metadata {
  const md = new grpc.Metadata();
  const auth = req.headers.authorization;
  if (auth) md.set('authorization', auth);
  return md;
}
