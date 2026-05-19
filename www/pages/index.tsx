import { useState } from 'react';

export default function Home() {
  const [input, setInput] = useState('');
  const [response, setResponse] = useState<string | null>(null);
  const [whoAmI, setWhoAmI] = useState<{
    mtls_peer_uri: string;
    user_id: string;
    user_email: string;
    trace_id: string;
  } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setResponse(null);

    try {
      const res = await fetch('/api/test-rpc', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name: input }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || 'Failed to call gRPC service');
      }

      setResponse(data.message);
      setWhoAmI(data.who_am_i ?? null);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{
      maxWidth: '600px',
      margin: '50px auto',
      padding: '20px',
      fontFamily: 'system-ui, sans-serif'
    }}>
      <h1>gRPC Rust & Node.js POC</h1>
      <p>This page calls a Rust gRPC server via a Next.js API route.</p>

      <form onSubmit={handleSubmit} style={{ marginTop: '30px' }}>
        <div style={{ marginBottom: '15px' }}>
          <label htmlFor="input" style={{ display: 'block', marginBottom: '5px' }}>
            Test Input:
          </label>
          <input
            id="input"
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Enter test message"
            style={{
              width: '100%',
              padding: '10px',
              fontSize: '16px',
              border: '1px solid #ccc',
              borderRadius: '4px',
            }}
            disabled={loading}
          />
        </div>

        <button
          type="submit"
          disabled={loading || !input.trim()}
          style={{
            padding: '10px 20px',
            fontSize: '16px',
            backgroundColor: loading ? '#ccc' : '#0070f3',
            color: 'white',
            border: 'none',
            borderRadius: '4px',
            cursor: loading ? 'not-allowed' : 'pointer',
          }}
        >
          {loading ? 'Calling...' : 'Call gRPC Server'}
        </button>
      </form>

      {error && (
        <div style={{
          marginTop: '20px',
          padding: '15px',
          backgroundColor: '#fee',
          border: '1px solid #fcc',
          borderRadius: '4px',
          color: '#c00',
        }}>
          <strong>Error:</strong> {error}
        </div>
      )}

      {response && (
        <div style={{
          marginTop: '20px',
          padding: '15px',
          backgroundColor: '#efe',
          border: '1px solid #cfc',
          borderRadius: '4px',
        }}>
          <strong>Response from Rust server:</strong>
          <div style={{ marginTop: '10px', fontSize: '18px' }}>{response}</div>
          {whoAmI && (
            <div style={{ marginTop: '15px', padding: '10px', backgroundColor: '#fff',
                          border: '1px solid #ddd', borderRadius: '4px', fontFamily: 'monospace',
                          fontSize: '13px' }}>
              <div style={{ fontWeight: 'bold', marginBottom: '6px', fontFamily: 'system-ui' }}>
                Mesh identity
              </div>
              <div>peer: <code>{whoAmI.mtls_peer_uri || '—'}</code></div>
              <div>user: <code>{whoAmI.user_id || '—'}</code>
                {whoAmI.user_email && <span style={{ color: '#666' }}> &lt;{whoAmI.user_email}&gt;</span>}
              </div>
              <div>trace: <code>{whoAmI.trace_id || '—'}</code></div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
