// Plain HTTP listener for Next.js. East-west mTLS termination is handled
// by the `www-inbound` ghostunnel sidecar sharing this container's
// netns. The sidecar listens on :3443 (TLS) and forwards plain to
// 127.0.0.1:3444 — this is :3444.
//
// Binds 127.0.0.1 only so only the sidecars in the shared netns can
// reach it. The container exposes no plaintext to the docker network.

import http from 'node:http';
import next from 'next';

const port = Number(process.env.WWW_PORT ?? 3444);
const hostname = '127.0.0.1';

const app = next({ dev: false, hostname, port });
const handle = app.getRequestHandler();

await app.prepare();

http.createServer((req, res) => handle(req, res)).listen(port, hostname, () => {
  console.log(`> Ready on http://${hostname}:${port}`);
});
