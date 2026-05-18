// LocalMesh outbound mTLS for www → server gRPC dials.
//
// Loads id.crt / id.key / trust.ca.crt from /run/www/ (the per-container
// cert dir produced by `make certs` from project.toml's [[services]]
// entry for `www`). Cached at module-load time — credentials are reused
// across requests.
//
// TODO: needs_prod_decisions per-call SVID rotation; SPIRE Workload API
// returns refreshing JWT/X.509 SVIDs in prod.

import * as grpc from '@grpc/grpc-js';
import fs from 'fs';

const CERTS_DIR = '/run/www';

let cached: grpc.ChannelCredentials | null = null;

export function meshChannelCredentials(): grpc.ChannelCredentials {
  if (cached) return cached;
  const rootCerts = fs.readFileSync(`${CERTS_DIR}/trust.ca.crt`);
  const privateKey = fs.readFileSync(`${CERTS_DIR}/id.key`);
  const certChain = fs.readFileSync(`${CERTS_DIR}/id.crt`);
  cached = grpc.credentials.createSsl(rootCerts, privateKey, certChain);
  return cached;
}
