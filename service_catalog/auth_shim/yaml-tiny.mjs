// Minimal YAML subset parser — just enough for .secrets/users.yaml's
// shape: a `users:` top-level key with a list of inline objects like
//   - { id: alice, email: alice@x.invalid, name: "Alice Example" }
//
// Trading a real YAML dep (js-yaml etc.) for ~30 lines so the auth-shim
// container's image is the stock node:22-alpine without an npm install
// step. If users.yaml grows beyond this shape, swap in js-yaml.

const STRIP = /^["']|["']$/g;

function parseInlineObj(s) {
  // {a: b, c: "d"}  →  {a: 'b', c: 'd'}
  const inner = s.trim().replace(/^\{|\}$/g, '');
  const out = {};
  for (const part of inner.split(',')) {
    const [k, ...rest] = part.split(':');
    if (!k) continue;
    const v = rest.join(':').trim().replace(STRIP, '');
    out[k.trim()] = v;
  }
  return out;
}

export function parse(text) {
  const lines = text.split('\n').map(l => l.replace(/#.*$/, '').trimEnd());
  const root = {};
  let listKey = null;
  for (const line of lines) {
    if (!line.trim()) continue;
    const m = /^(\w+):\s*$/.exec(line);
    if (m) { listKey = m[1]; root[listKey] = []; continue; }
    const item = /^\s+-\s+(\{.*\})\s*$/.exec(line);
    if (item && listKey) { root[listKey].push(parseInlineObj(item[1])); continue; }
  }
  return root;
}
