import { Routes, Route } from 'react-router-dom';
import { toast } from 'sonner';
import { useServerStatus } from './hooks/useServerStatus';
import { useStartServer, useStopServer } from './hooks/useServerMutations';
import { useAuth } from './hooks/useAuth';
import { ProtectedRoute } from './components/ProtectedRoute';
import type { ServerState } from './types/server';

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
 * Copies the server IP to clipboard with toast feedback
 */
async function handleCopyIP(ip: string) {
  try {
    await navigator.clipboard.writeText(ip);
    toast.success('IP copied to clipboard');
  } catch {
    toast.error('Failed to copy IP');
  }
}

/**
 * Header component with user info and logout
 */
function Header({ email }: { email: string }) {
  return (
    <div className="page-header">
      <div>
        <h1 className="title">Metio</h1>
        <p className="subtitle">Minecraft Server Controller</p>
      </div>
      <div className="header-user">
        <span className="user-email">{email}</span>
        <a href="/auth/logout" className="btn btn-outline btn-sm">
          Logout
        </a>
      </div>
    </div>
  );
}

/**
 * Dashboard component - main server control panel
 */
function Dashboard() {
  const { data: status, isLoading, error } = useServerStatus();
  const { user } = useAuth();
  const startMutation = useStartServer();
  const stopMutation = useStopServer();

  const isMutating = startMutation.isPending || stopMutation.isPending;

  // Loading state
  if (isLoading) {
    return (
      <div className="dark min-h-screen bg-background p-8">
        <div className="container">
          <Header email={user?.email || ''} />
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
          <Header email={user?.email || ''} />
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
          <Header email={user?.email || ''} />
          <div className="card">
            <div className="card-content">
              <p className="text-muted">No server status available</p>
              <div className="controls mt-4">
                <button
                  className="btn btn-green"
                  onClick={() => startMutation.mutate()}
                  disabled={isMutating}
                >
                  {startMutation.isPending ? 'Starting...' : 'Start Server'}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  const badge = getStatusBadge(status.status);
  const isRunning = status.status === 'RUNNING';
  const isStopped = status.status === 'STOPPED';
  const isTransitioning = status.status === 'STARTING' || status.status === 'STOPPING';

  return (
    <div className="dark min-h-screen bg-background p-8">
      <div className="container">
        <Header email={user?.email || ''} />

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
                <span className="stat-value">{formatServerState(status.status)}</span>
              </div>
              <div className="stat">
                <span className="stat-label">Players</span>
                <span className="stat-value">
                  {status.players}/{status.maxPlayers}
                </span>
              </div>
              <div className="stat">
                <span className="stat-label">Uptime</span>
                <span className="stat-value">{status.uptime || '-'}</span>
              </div>
              <div className="stat">
                <span className="stat-label">IP</span>
                <span className="stat-value">{status.ip || '-'}</span>
              </div>
            </div>

            <div className="separator" />

            <div className="controls">
              {isRunning && (
                <button
                  className="btn btn-red"
                  onClick={() => stopMutation.mutate()}
                  disabled={isMutating}
                >
                  {stopMutation.isPending ? 'Stopping...' : 'Stop Server'}
                </button>
              )}
              {isStopped && (
                <button
                  className="btn btn-green"
                  onClick={() => startMutation.mutate()}
                  disabled={isMutating}
                >
                  {startMutation.isPending ? 'Starting...' : 'Start Server'}
                </button>
              )}
              {isTransitioning && (
                <button className="btn btn-outline" disabled>
                  {status.status === 'STARTING' ? 'Starting...' : 'Stopping...'}
                </button>
              )}
              {status.ip && (
                <button
                  className="btn btn-outline"
                  onClick={() => handleCopyIP(status.ip)}
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

/**
 * Main App component with routing
 */
function App() {
  return (
    <Routes>
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Dashboard />
          </ProtectedRoute>
        }
      />
    </Routes>
  );
}

export default App;
