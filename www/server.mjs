// Plain HTTP listener for Next.js. East-west mTLS is handled by the
// kuma-dp dataplane sharing this container's netns; its transparent-proxy
// iptables intercept inbound on :3443, terminate mTLS, and forward here.
// Bind 0.0.0.0 (not loopback) so the dataplane's inbound listener — which
// forwards to this container's pod IP — can reach the app. Network
// isolation comes from the dataplane's iptables, not the bind interface.

import http from 'node:http';
import next from 'next';

const port = Number(process.env.WWW_PORT ?? 3443);
const hostname = '0.0.0.0';

const app = next({ dev: false, hostname, port });
const handle = app.getRequestHandler();

await app.prepare();

http.createServer((req, res) => handle(req, res)).listen(port, hostname, () => {
  console.log(`> Ready on http://${hostname}:${port}`);
});
