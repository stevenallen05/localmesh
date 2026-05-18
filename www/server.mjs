// HTTPS listener wrapping the Next.js handler so the east-west hop from
// Caddy → www is mTLS. Replaces `next start` (which listens on plaintext
// HTTP) as the container's entrypoint.
//
// Loads /run/www/{id.crt,id.key,trust.ca.crt} per the LocalMesh cert
// dir convention. Verify-CA: any client presenting a cert chained to the
// LocalMesh CA is accepted; identity ENRICHMENT happens at the app layer
// when WhoAmI lands.
//
// TODO: needs_prod_decisions Node-as-mTLS-terminator is dev-only; prod's
// ingress controller terminates north-south, sidecar terminates east-west.

import https from 'node:https';
import fs from 'node:fs';
import next from 'next';

const CERTS_DIR = '/run/www';
const port = Number(process.env.WWW_PORT ?? 3443);
const hostname = '0.0.0.0';

const app = next({
  dev: false,
  hostname,
  port,
});
const handle = app.getRequestHandler();

await app.prepare();

const tlsOpts = {
  cert: fs.readFileSync(`${CERTS_DIR}/id.crt`),
  key: fs.readFileSync(`${CERTS_DIR}/id.key`),
  ca: fs.readFileSync(`${CERTS_DIR}/trust.ca.crt`),
  requestCert: true,
  rejectUnauthorized: true,
};

https.createServer(tlsOpts, (req, res) => handle(req, res)).listen(port, () => {
  console.log(`> Ready on https://${hostname}:${port}`);
});
