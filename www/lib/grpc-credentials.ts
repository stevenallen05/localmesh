// Plain gRPC credentials. East-west mTLS is handled by the
// `www-outbound` ghostunnel sidecar — www dials it on 127.0.0.1:50443
// plaintext; the sidecar wraps mTLS to server:50051.
//
// Kept as a one-line indirection so a future TLS re-introduction stays
// at this single site rather than scattering grpc.credentials calls
// across handlers.

import * as grpc from '@grpc/grpc-js';

export function meshChannelCredentials(): grpc.ChannelCredentials {
  return grpc.credentials.createInsecure();
}
