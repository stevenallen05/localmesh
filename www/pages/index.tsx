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

  const [catalogLoading, setCatalogLoading] = useState(false);
  const [catalogError, setCatalogError] = useState<string | null>(null);
  const [catalogOk, setCatalogOk] = useState(false);

  const [pgLoading, setPgLoading] = useState(false);
  const [pgError, setPgError] = useState<string | null>(null);
  const [pgOk, setPgOk] = useState(false);

  const handlePrintPostgresStats = async () => {
    setPgLoading(true);
    setPgError(null);
    setPgOk(false);
    try {
      const res = await fetch('/api/print-postgres-stats', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed');
      console.log('postgres stats:', data);
      setPgOk(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setPgError(message);
    } finally {
      setPgLoading(false);
    }
  };

  const handleListCatalog = async () => {
    setCatalogLoading(true);
    setCatalogError(null);
    setCatalogOk(false);
    try {
      const res = await fetch('/api/list-metrics-catalog', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed');
      console.log('metrics catalog:', data);
      setCatalogOk(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setCatalogError(message);
    } finally {
      setCatalogLoading(false);
    }
  };

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

      <div style={{ marginTop: '30px' }}>
        <button
          type="button"
          onClick={handleListCatalog}
          disabled={catalogLoading}
          style={{
            padding: '10px 20px',
            fontSize: '16px',
            backgroundColor: catalogLoading ? '#ccc' : '#0070f3',
            color: 'white',
            border: 'none',
            borderRadius: '4px',
            cursor: catalogLoading ? 'not-allowed' : 'pointer',
          }}
        >
          {catalogLoading ? 'Loading...' : 'List Grafana datasources + metrics'}
        </button>

        {catalogOk && (
          <div style={{ marginTop: '15px', padding: '10px',
                        backgroundColor: '#efe', borderRadius: '4px' }}>
            Logged to browser console and <code>docker compose logs www</code>.
          </div>
        )}
        {catalogError && (
          <div style={{ marginTop: '15px', padding: '10px',
                        backgroundColor: '#fee', borderRadius: '4px', color: '#c00' }}>
            <strong>Error:</strong> {catalogError}
          </div>
        )}
      </div>

      <div style={{ marginTop: '30px' }}>
        <button
          type="button"
          onClick={handlePrintPostgresStats}
          disabled={pgLoading}
          style={{
            padding: '10px 20px',
            fontSize: '16px',
            backgroundColor: pgLoading ? '#ccc' : '#0070f3',
            color: 'white',
            border: 'none',
            borderRadius: '4px',
            cursor: pgLoading ? 'not-allowed' : 'pointer',
          }}
        >
          {pgLoading ? 'Loading...' : 'Print Postgres stats to console'}
        </button>

        {pgOk && (
          <div style={{ marginTop: '15px', padding: '10px',
                        backgroundColor: '#efe', borderRadius: '4px' }}>
            Logged to browser console.
          </div>
        )}
        {pgError && (
          <div style={{ marginTop: '15px', padding: '10px',
                        backgroundColor: '#fee', borderRadius: '4px', color: '#c00' }}>
            <strong>Error:</strong> {pgError}
          </div>
        )}
      </div>

      <div style={{ marginTop: '40px', padding: '15px', backgroundColor: '#f5f5f5', borderRadius: '4px' }}>
        <h3>How it works:</h3>
        <ol>
          <li>Enter text in the input field above</li>
          <li>Click "Call gRPC Server" to send a request</li>
          <li>The Next.js API route calls the Rust gRPC server</li>
          <li>The Rust server processes the request and returns a response</li>
          <li>The response is displayed above</li>
        </ol>
      </div>
    </div>
  );
}




