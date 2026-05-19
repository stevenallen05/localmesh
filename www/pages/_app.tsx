// LocalMesh top-of-page banner: shows the logged-in user, read from
// oauth2-proxy's X-Forwarded-* headers on the initial server-render
// pass. www is identity-free — no cookies, no env fallback, no
// client-side fetch. The actual identity lives in the JWT proxied
// to server.

import type { AppContext, AppProps } from 'next/app';
import App from 'next/app';

// Node lowercases header names. Each header is string | string[] | undefined;
// multi-value (string[]) is rare for these headers but possible — pick first.
function pick(v: string | string[] | undefined): string {
  return Array.isArray(v) ? v[0] ?? '' : v ?? '';
}

type BannerProps = { name: string; email: string };

function UserBanner({ name, email }: BannerProps) {
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
        Logged in as: <strong>{name || 'Authenticated user'}</strong>
        {email && <span style={{ color: '#666' }}> &lt;{email}&gt;</span>}
      </span>
      <a href="/oauth2/sign_out?rd=/">[Sign out]</a>
    </div>
  );
}

type MyAppProps = AppProps & BannerProps;

export default function MyApp({ Component, pageProps, name, email }: MyAppProps) {
  return (
    <>
      <UserBanner name={name} email={email} />
      <Component {...pageProps} />
    </>
  );
}

// ctx.ctx.req is only set on the server pass (initial page load via Caddy).
// On client-side navigations there is no req — the SSR-computed value is
// reused, which is fine because the logged-in identity is session-stable.
MyApp.getInitialProps = async (appCtx: AppContext) => {
  const appProps = await App.getInitialProps(appCtx);
  const h = appCtx.ctx.req?.headers ?? {};
  return {
    ...appProps,
    name:  pick(h['x-forwarded-preferred-username']),
    email: pick(h['x-forwarded-email']),
  };
};
