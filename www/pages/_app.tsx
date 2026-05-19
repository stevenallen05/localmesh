// LocalMesh top-of-page banner: shows the logged-in user (read via
// /api/whoami, which exposes oauth2-proxy's X-Forwarded-* headers
// for display only) and a sign-out link to oauth2-proxy.
//
// www is identity-free — no cookies, no env fallback, no header
// parsing. The actual identity lives in the JWT proxied to server.

import type { AppProps } from 'next/app';
import { useEffect, useState } from 'react';

function UserBanner() {
  const [name,  setName]  = useState<string>('—');
  const [email, setEmail] = useState<string>('');

  useEffect(() => {
    fetch('/api/whoami')
      .then(r => r.json())
      .then(({ name, email }) => {
        setName(name || 'Authenticated user');
        setEmail(email || '');
      })
      .catch(() => { /* unauthenticated request can't reach here post-cutover */ });
  }, []);

  return (
    <div
      style={{
        padding: '8px 16px',
        background: '#f5f5f5',
        borderBottom: '1px solid #ddd',
        fontFamily: 'monospace',
        fontSize: 14,
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
      }}
    >
      <span>
        Logged in as: <strong>{name}</strong>
        {email && <span style={{ color: '#666' }}> &lt;{email}&gt;</span>}
      </span>
      <a href="/oauth2/sign_out?rd=/">[Sign out]</a>
    </div>
  );
}

export default function App({ Component, pageProps }: AppProps) {
  return (
    <>
      <UserBanner />
      <Component {...pageProps} />
    </>
  );
}
