import { useServerStatus } from './hooks/useServerStatus';
import type { ServerState } from './types/firestore';

/**
 * Maps server state to badge CSS class and label
 */
function getStatusBadge(state: ServerState): { className: string; label: string } {
  switch (state) {
    case 'RUNNING':
      return { className: 'badge-online', label: 'Online' };
    case 'STOPPED':
      return { className: 'badge-offline', label: 'Offline' };
    case 'STARTING':
    case 'STOPPING':
      return { className: 'badge-transitioning', label: state === 'STARTING' ? 'Starting...' : 'Stopping...' };
    default:
      return { className: 'badge-offline', label: 'Unknown' };
  }
}

/**
 * Formats server state for display
 */
function formatServerState(state: ServerState): string {
  switch (state) {
    case 'RUNNING':
      return 'Running';
    case 'STOPPED':
      return 'Stopped';
    case 'STARTING':
      return 'Starting';
    case 'STOPPING':
      return 'Stopping';
    default:
      return 'Unknown';
  }
}

/**
 * Handles server start action
 */
async function handleStartServer() {
  try {
    const response = await fetch('/api/server/start', { method: 'POST' });
    if (!response.ok) {
      console.error('Failed to start server');
    }
  } catch (err) {
    console.error('Error starting server:', err);
  }
}

/**
 * Handles server stop action
 */
async function handleStopServer() {
  try {
    const response = await fetch('/api/server/stop', { method: 'POST' });
    if (!response.ok) {
      console.error('Failed to stop server');
    }
  } catch (err) {
    console.error('Error stopping server:', err);
  }
}

/**
 * Copies the server IP to clipboard
 */
async function handleCopyIP(ip: string) {
  try {
    await navigator.clipboard.writeText(ip);
  } catch (err) {
    console.error('Failed to copy IP:', err);
  }
}

function App() {
  const { status, loading, error } = useServerStatus();

  // Loading state
  if (loading) {
    return (
      <div className="dark min-h-screen bg-background p-8">
        <div className="container">
          <div className="page-header">
            <h1 className="title">Metio</h1>
            <p className="subtitle">Minecraft Server Controller</p>
          </div>
          <div className="card">
            <div className="card-content">
              <p className="text-muted">Loading server status...</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="dark min-h-screen bg-background p-8">
        <div className="container">
          <div className="page-header">
            <h1 className="title">Metio</h1>
            <p className="subtitle">Minecraft Server Controller</p>
          </div>
          <div className="card">
            <div className="card-content">
              <p className="text-red-500">Error: {error.message}</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // No status available
  if (!status) {
    return (
      <div className="dark min-h-screen bg-background p-8">
        <div className="container">
          <div className="page-header">
            <h1 className="title">Metio</h1>
            <p className="subtitle">Minecraft Server Controller</p>
          </div>
          <div className="card">
            <div className="card-content">
              <p className="text-muted">No server status available</p>
              <div className="controls mt-4">
                <button className="btn btn-green" onClick={handleStartServer}>
                  Start Server
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  const badge = getStatusBadge(status.server_state);
  const isRunning = status.server_state === 'RUNNING';
  const isStopped = status.server_state === 'STOPPED';
  const isTransitioning = status.server_state === 'STARTING' || status.server_state === 'STOPPING';

  return (
    <div className="dark min-h-screen bg-background p-8">
      <div className="container">
        <div className="page-header">
          <h1 className="title">Metio</h1>
          <p className="subtitle">Minecraft Server Controller</p>
        </div>

        <div className="card">
          <div className="card-header">
            <h2 className="card-title">
              Server Status
              <span className={`badge ${badge.className}`}>{badge.label}</span>
            </h2>
          </div>
          <div className="card-content">
            <div className="stats-grid">
              <div className="stat">
                <span className="stat-label">Status</span>
                <span className="stat-value">{formatServerState(status.server_state)}</span>
              </div>
              <div className="stat">
                <span className="stat-label">Players</span>
                <span className="stat-value">
                  {status.players.current}/{status.players.max}
                </span>
              </div>
              <div className="stat">
                <span className="stat-label">Uptime</span>
                <span className="stat-value">{status.uptime || '-'}</span>
              </div>
              <div className="stat">
                <span className="stat-label">IP</span>
                <span className="stat-value">{status.instance_ip || '-'}</span>
              </div>
            </div>

            <div className="separator" />

            <div className="controls">
              {isRunning && (
                <button className="btn btn-red" onClick={handleStopServer}>
                  Stop Server
                </button>
              )}
              {isStopped && (
                <button className="btn btn-green" onClick={handleStartServer}>
                  Start Server
                </button>
              )}
              {isTransitioning && (
                <button className="btn btn-outline" disabled>
                  {status.server_state === 'STARTING' ? 'Starting...' : 'Stopping...'}
                </button>
              )}
              {status.instance_ip && (
                <button
                  className="btn btn-outline"
                  onClick={() => handleCopyIP(status.instance_ip)}
                >
                  Copy IP
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default App;
