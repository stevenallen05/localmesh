import type { GetServerSideProps } from 'next';
import { useState } from 'react';

type DevUser = { id: string; email: string; name: string };

export const getServerSideProps: GetServerSideProps = async () => {
  try {
    const r = await fetch('http://auth-shim:8080/users');
    const { users } = await r.json();
    return { props: { users: users ?? [] } };
  } catch {
    // auth_shim plugin not included — fall back to env-based identity.
    return { props: { users: [] } };
  }
};

export default function Login({ users }: { users: DevUser[] }) {
  const [picked, setPicked] = useState<string>('');
  const submit = () => {
    const u = users.find(x => x.id === picked);
    if (!u) return;
    const set = (k: string, v: string) =>
      (document.cookie = `${k}=${encodeURIComponent(v)}; path=/`);
    set('lm.user.id', u.id);
    set('lm.user.email', u.email);
    set('lm.user.name', u.name);
    window.location.href = '/';
  };
  return (
    <main style={{ padding: 32, fontFamily: 'system-ui', maxWidth: 480, margin: '0 auto' }}>
      <h1>LocalMesh dev login</h1>
      {users.length === 0 ? (
        <p>
          No users available — auth_shim plugin isn't running. Set <code>DEV_USER_ID</code>,{' '}
          <code>DEV_USER_EMAIL</code>, <code>DEV_USER_NAME</code> in <code>.env</code> for the fallback identity.
        </p>
      ) : (
        <>
          <p>Pick a user to log in as. (Sets dev cookies; not real auth.)</p>
          {users.map(u => (
            <label key={u.id} style={{ display: 'block', margin: '8px 0' }}>
              <input
                type="radio"
                name="user"
                value={u.id}
                checked={picked === u.id}
                onChange={() => setPicked(u.id)}
              />{' '}
              {u.name} <code style={{ color: '#666' }}>&lt;{u.email}&gt;</code>
            </label>
          ))}
          <button onClick={submit} disabled={!picked} style={{ marginTop: 12 }}>
            Continue
          </button>
        </>
      )}
    </main>
  );
}
