// LocalMesh user identity at the www edge.
//
// PII-at-ingress rule (docs/superpowers/specs/2026-05-18-localmesh-
// service-mesh-design.md §9.3): user.* / enduser.* attributes land on
// the OTel root span at the API-route entry only. Downstream services
// receive user identity via gRPC metadata into request extensions for
// business-logic use, but must not echo PII onto their own OTel spans
// or indexed Loki labels. Enforcement is documented + code-reviewed
// today; mechanical enforcement (Vector / Tempo scrubbing) is
// deferred — TODO: needs_prod_decisions PII enforcement (regex
// scrubbing at ingest? lint check on .setAttributes call sites?).

import type { NextApiRequest } from 'next';
import type { IncomingMessage } from 'http';
import * as grpc from '@grpc/grpc-js';

export type User = { id: string; email: string; name: string };

function parseCookies(req: NextApiRequest | IncomingMessage): Record<string, string> {
  const header = req.headers.cookie ?? '';
  return Object.fromEntries(
    header.split(';').map(s => s.trim()).filter(Boolean).map(kv => {
      const i = kv.indexOf('=');
      if (i === -1) return [kv, ''];
      return [decodeURIComponent(kv.slice(0, i)), decodeURIComponent(kv.slice(i + 1))];
    })
  );
}

function pickHeader(req: NextApiRequest | IncomingMessage, k: string): string | undefined {
  const v = req.headers[k.toLowerCase()];
  return Array.isArray(v) ? v[0] : v;
}

// Identity resolution order: cookie (dev picker) > X-Forwarded-* (real
// forward-auth proxy in prod) > DEV_USER_* env (fallback for unauthed dev).
export function getUser(req: NextApiRequest | IncomingMessage): User {
  const c = parseCookies(req);
  return {
    id:    c['lm.user.id']    ?? pickHeader(req, 'X-Forwarded-User')               ?? process.env.DEV_USER_ID    ?? 'dev-user',
    email: c['lm.user.email'] ?? pickHeader(req, 'X-Forwarded-Email')              ?? process.env.DEV_USER_EMAIL ?? 'dev-user@example.invalid',
    name:  c['lm.user.name']  ?? pickHeader(req, 'X-Forwarded-Preferred-Username') ?? process.env.DEV_USER_NAME  ?? 'Dev User',
  };
}

export function userMetadata(user: User): grpc.Metadata {
  const md = new grpc.Metadata();
  md.set('x-user-id', user.id);
  md.set('x-user-email', user.email);
  md.set('x-user-name', user.name);
  return md;
}
