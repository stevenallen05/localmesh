// Returns the logged-in user's email + name from oauth2-proxy's
// X-Forwarded-* headers. Used by _app.tsx's UserBanner for display
// only — the actual identity lives in the JWT proxied to server.
//
// Identity-free at the layer below: this route trusts the proxy
// (every request reaches www via oauth2-proxy + Caddy forward_auth).

import type { NextApiRequest, NextApiResponse } from 'next';

// Node lowercases header names. Each header is string | string[] | undefined;
// multi-value (string[]) is rare for these headers but possible — pick first.
function pick(v: string | string[] | undefined): string {
  return Array.isArray(v) ? v[0] ?? '' : v ?? '';
}

export default function handler(req: NextApiRequest, res: NextApiResponse) {
  res.status(200).json({
    email: pick(req.headers['x-forwarded-email']),
    name:  pick(req.headers['x-forwarded-preferred-username']),
  });
}
