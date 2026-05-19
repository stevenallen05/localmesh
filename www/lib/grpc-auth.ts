// Forward identity from the ingress to the server as gRPC metadata.
//
// oauth2-proxy at the Caddy ingress validates the IdP JWT once and sets
// X-Forwarded-{User,Email,Preferred-Username} plus Authorization on
// every gated request. www does not validate or parse — it copies those
// headers into gRPC metadata (`x-user-id` / `x-user-email` /
// `x-user-name` / `authorization`). Server trusts the metadata because
// the mTLS channel was gated by ghostunnel's --allow-uri allowlist.

import type { NextApiRequest } from 'next';
import type { IncomingMessage } from 'http';
import * as grpc from '@grpc/grpc-js';

function pick(v: string | string[] | undefined): string | undefined {
  const s = Array.isArray(v) ? v[0] : v;
  if (!s) return undefined;
  // Belt-and-braces against a Caddy `{http.…}` placeholder leak — same
  // sanitiser shape as pages/_app.tsx.
  return s.startsWith('{http.') ? undefined : s;
}

export function forwardAuth(req: NextApiRequest | IncomingMessage): grpc.Metadata {
  const md = new grpc.Metadata();
  const auth = req.headers.authorization;
  if (auth) md.set('authorization', auth);
  const id = pick(req.headers['x-forwarded-user']);
  const email = pick(req.headers['x-forwarded-email']);
  const name = pick(req.headers['x-forwarded-preferred-username']);
  if (id) md.set('x-user-id', id);
  if (email) md.set('x-user-email', email);
  if (name) md.set('x-user-name', name);
  return md;
}
