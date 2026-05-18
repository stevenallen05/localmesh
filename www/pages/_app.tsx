// LocalMesh top-of-page banner: shows the logged-in dev user (from the
// `lm.user.*` cookies set by /auth/login) and a switcher link.
//
// Identity ends here for the UI side; the server-side `getUser(req)`
// helper in www/lib/identity.ts is what actually threads user info into
// the API routes and downstream gRPC metadata.

import type { AppProps } from 'next/app';
import { useEffect, useState } from 'react';

function decodeCookie(raw: string, key: string): string | undefined {
  const m = raw.split(';').map(s => s.trim()).find(s => s.startsWith(`${key}=`));
  if (!m) return undefined;
  try {
    return decodeURIComponent(m.slice(key.length + 1));
  } catch {
    return undefined;
  }
}

function UserBanner() {
  const [name, setName] = useState<string>('—');
  const [email, setEmail] = useState<string>('');

  useEffect(() => {
    const raw = document.cookie;
    setName(decodeCookie(raw, 'lm.user.name') ?? 'Dev User');
    setEmail(decodeCookie(raw, 'lm.user.email') ?? '');
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
      <a href="/auth/login">[Switch user]</a>
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
