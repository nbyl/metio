import { toast } from 'sonner';
import { useServerStatus } from '../../hooks/useServerStatus';
import { useStartServer, useStopServer } from '../../hooks/useServerMutations';
import { Card, CardHeader, CardTitle, CardContent } from '../ui/Card';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import { Separator } from '../ui/Separator';
import { Skeleton } from '../ui/Skeleton';
import { StatsGrid, type StatItem } from '../layout/StatsGrid';
import type { ServerState } from '../../types/server';
import { cn } from '../../lib/utils';

export interface ServerStatusCardProps {
  /** Additional CSS classes */
  className?: string;
}

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
 * Loading skeleton for the server status card.
 * Shows placeholder content while data is being fetched.
 */
function ServerStatusCardSkeleton({ className }: { className?: string }) {
  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>
          <Skeleton className="h-6 w-32" />
          <Skeleton className="h-5 w-16" />
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="stats-grid">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="stat">
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-5 w-20 mt-1" />
            </div>
          ))}
        </div>
        <Separator />
      </CardContent>
    </Card>
  );
}

/**
 * ServerStatusCard displays real-time server information.
 *
 * States:
 * - Loading: Shows skeleton placeholder
 * - Error: Shows error message with retry button
 * - Stopped: Shows header with badge and start button (no stats)
 * - Running/Transitioning: Shows full stats grid with controls
 *
 * @example
 * ```tsx
 * <ServerStatusCard />
 * ```
 */
export function ServerStatusCard({ className }: ServerStatusCardProps) {
  const { data: status, isLoading, error, refetch } = useServerStatus();
  const startMutation = useStartServer();
  const stopMutation = useStopServer();

  const isMutating = startMutation.isPending || stopMutation.isPending;

  // Loading state - show skeleton
  if (isLoading) {
    return <ServerStatusCardSkeleton className={className} />;
  }

  // Error state - show error with retry
  if (error) {
    return (
      <Card className={className}>
        <CardContent>
          <p className="text-red-500">Error: {error.message}</p>
          <div className="controls mt-4">
            <Button variant="outline" onClick={() => refetch()}>
              Retry
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  // No status available - show start button
  if (!status) {
    return (
      <Card className={className}>
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
    );
  }

  const isRunning = status.status === 'RUNNING';
  const isStopped = status.status === 'STOPPED';
  const isTransitioning =
    status.status === 'STARTING' || status.status === 'STOPPING';

  // Only show stats when server is not stopped
  const showStats = !isStopped;

  const stats: StatItem[] = showStats
    ? [
        { label: 'Version', value: status.version || '-' },
        { label: 'Players', value: `${status.players}/${status.maxPlayers}` },
        { label: 'Uptime', value: status.uptime || '-' },
        { label: 'IP', value: status.ip || '-' },
      ]
    : [];

  return (
    <Card className={cn(className)}>
      <CardHeader>
        <CardTitle>
          Server Status
          <Badge variant={getStatusBadgeVariant(status.status)}>
            {getStatusLabel(status.status)}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {showStats && (
          <>
            <StatsGrid stats={stats} />
            <Separator />
          </>
        )}

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
  );
}
