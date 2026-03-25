import { Routes, Route } from 'react-router-dom';
import { toast } from 'sonner';
import { useServerStatus } from './hooks/useServerStatus';
import { useStartServer, useStopServer } from './hooks/useServerMutations';
import { useAuth } from './hooks/useAuth';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Layout } from './components/layout/Layout';
import { Header } from './components/layout/Header';
import { StatsGrid, type StatItem } from './components/layout/StatsGrid';
import { Card, CardHeader, CardTitle, CardContent } from './components/ui/Card';
import { Button } from './components/ui/Button';
import { Badge } from './components/ui/Badge';
import { Separator } from './components/ui/Separator';
import type { ServerState } from './types/server';

/**
 * Maps server state to badge variant
 */
function getStatusBadgeVariant(
  state: ServerState
): 'online' | 'offline' | 'transitioning' {
  switch (state) {
    case 'RUNNING':
      return 'online';
    case 'STOPPED':
      return 'offline';
    case 'STARTING':
    case 'STOPPING':
      return 'transitioning';
    default:
      return 'offline';
  }
}

/**
 * Maps server state to display label
 */
function getStatusLabel(state: ServerState): string {
  switch (state) {
    case 'RUNNING':
      return 'Online';
    case 'STOPPED':
      return 'Offline';
    case 'STARTING':
      return 'Starting...';
    case 'STOPPING':
      return 'Stopping...';
    default:
      return 'Unknown';
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
      <Layout>
        <Header email={user?.email} showUser />
        <Card>
          <CardContent>
            <p className="text-muted">Loading server status...</p>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  // Error state
  if (error) {
    return (
      <Layout>
        <Header email={user?.email} showUser />
        <Card>
          <CardContent>
            <p className="text-red-500">Error: {error.message}</p>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  // No status available
  if (!status) {
    return (
      <Layout>
        <Header email={user?.email} showUser />
        <Card>
          <CardContent>
            <p className="text-muted">No server status available</p>
            <div className="controls mt-4">
              <Button
                variant="primary"
                onClick={() => startMutation.mutate()}
                disabled={isMutating}
                loading={startMutation.isPending}
              >
                Start Server
              </Button>
            </div>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  const isRunning = status.status === 'RUNNING';
  const isStopped = status.status === 'STOPPED';
  const isTransitioning =
    status.status === 'STARTING' || status.status === 'STOPPING';

  const stats: StatItem[] = [
    { label: 'Status', value: formatServerState(status.status) },
    { label: 'Players', value: `${status.players}/${status.maxPlayers}` },
    { label: 'Uptime', value: status.uptime || '-' },
    { label: 'IP', value: status.ip || '-' },
  ];

  return (
    <Layout>
      <Header email={user?.email} showUser />

      <Card>
        <CardHeader>
          <CardTitle>
            Server Status
            <Badge variant={getStatusBadgeVariant(status.status)}>
              {getStatusLabel(status.status)}
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <StatsGrid stats={stats} />

          <Separator />

          <div className="controls">
            {isRunning && (
              <Button
                variant="danger"
                onClick={() => stopMutation.mutate()}
                disabled={isMutating}
                loading={stopMutation.isPending}
              >
                Stop Server
              </Button>
            )}
            {isStopped && (
              <Button
                variant="primary"
                onClick={() => startMutation.mutate()}
                disabled={isMutating}
                loading={startMutation.isPending}
              >
                Start Server
              </Button>
            )}
            {isTransitioning && (
              <Button variant="outline" disabled>
                {status.status === 'STARTING' ? 'Starting...' : 'Stopping...'}
              </Button>
            )}
            {status.ip && (
              <Button variant="outline" onClick={() => handleCopyIP(status.ip)}>
                Copy IP
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </Layout>
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
