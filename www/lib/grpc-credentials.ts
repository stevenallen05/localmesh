// Plain gRPC credentials. East-west mTLS is handled by the local envoy
// sidecar — www dials `server:50051` plaintext, iptables redirects to the
// envoy outbound listener, and envoy origins mTLS to server-mesh:50051.
//
// Kept as a one-line indirection so a future TLS re-introduction stays
// at this single site rather than scattering grpc.credentials calls
// across handlers.

import * as grpc from '@grpc/grpc-js';

export function meshChannelCredentials(): grpc.ChannelCredentials {
  return grpc.credentials.createInsecure();
}
