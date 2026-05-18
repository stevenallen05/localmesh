// auth-shim: dev-only user picker source. Reads the list of dev users
// from /etc/users.yaml (bind-mounted from .secrets/users.yaml) and
// exposes GET /users → JSON. www's /auth/login page fetches this server-
// side and renders a picker; user identity flows from there via cookies.
//
// Not part of the mesh — plugin.toml has mesh_exempt = true.
//
// TODO: needs_prod_decisions oauth2-proxy + IdP (Dex / Keycloak / etc.)
// in prod; this stub is the dev-only stand-in.

import http from 'node:http';
import fs from 'node:fs';
import { parse as parseYaml } from './yaml-tiny.mjs';

const usersFile = process.env.USERS_FILE || '/etc/users.yaml';

let users = [];
try {
  users = parseYaml(fs.readFileSync(usersFile, 'utf8')).users ?? [];
} catch (err) {
  console.error(`auth-shim: failed to load ${usersFile}:`, err.message);
}

http.createServer((req, res) => {
  if (req.url === '/users') {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ users }));
    return;
  }
  if (req.url === '/healthz') {
    res.writeHead(200, { 'content-type': 'text/plain' });
    res.end('ok');
    return;
  }
  res.writeHead(404); res.end();
}).listen(8080, () => {
  console.log(`auth-shim listening :8080 — ${users.length} dev user(s) loaded`);
});
