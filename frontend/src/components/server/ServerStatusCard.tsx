import { useState } from 'react';
import { toast } from 'sonner';
import { Copy, Check, ChevronDown, ChevronRight, X, Loader2 } from 'lucide-react';
import { useServerStatus } from '../../hooks/useServerStatus';
import { useStartServer, useStopServer } from '../../hooks/useServerMutations';
import {
  useWhitelist,
  useAddPlayer,
  useRemovePlayer,
  useToggleWhitelist,
} from '../../hooks/useWhitelist';
import { useCopyToClipboard } from '../../hooks/useCopyToClipboard';
import { Card, CardHeader, CardTitle, CardContent } from '../ui/Card';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import { Separator } from '../ui/Separator';
import { Skeleton } from '../ui/Skeleton';
import { Tooltip } from '../ui/Tooltip';
import { Switch } from '../ui/Switch';
import { StatsGrid, type StatItem } from '../layout/StatsGrid';
import type { ServerState } from '../../types/server';
import type { WhitelistPlayer } from '../../types/whitelist';
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
 * WhitelistSection displays whitelist management controls
 */
interface WhitelistSectionProps {
  isRunning: boolean;
}

function WhitelistSection({ isRunning }: WhitelistSectionProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [newUsername, setNewUsername] = useState('');

  const { data: whitelist, isLoading } = useWhitelist();
  const addPlayerMutation = useAddPlayer();
  const removePlayerMutation = useRemovePlayer();
  const toggleWhitelistMutation = useToggleWhitelist();

  if (!isRunning) {
    return null;
  }

  const handleAddPlayer = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUsername.trim()) return;

    addPlayerMutation.mutate(newUsername.trim(), {
      onSuccess: () => setNewUsername(''),
    });
  };

  const handleToggle = (enabled: boolean) => {
    toggleWhitelistMutation.mutate(enabled);
  };

  const handleRemovePlayer = (player: WhitelistPlayer) => {
    removePlayerMutation.mutate(player.uuid);
  };

  const isToggling = toggleWhitelistMutation.isPending;
  const isAdding = addPlayerMutation.isPending;

  return (
    <div className="whitelist-section">
      <div className="whitelist-header">
        <button
          type="button"
          className="whitelist-title"
          onClick={() => setIsExpanded(!isExpanded)}
        >
          {isExpanded ? (
            <ChevronDown className="h-4 w-4" />
          ) : (
            <ChevronRight className="h-4 w-4" />
          )}
          Whitelist
          {whitelist && (
            <span className="text-slate-500">({whitelist.players.length})</span>
          )}
        </button>
        {whitelist && (
          <Switch
            checked={whitelist.enabled}
            onChange={handleToggle}
            disabled={isToggling}
            aria-label="Toggle whitelist"
          />
        )}
      </div>

      {isExpanded && (
        <div>
          {isLoading ? (
            <div className="flex items-center justify-center py-4">
              <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
            </div>
          ) : (
            <>
              <form onSubmit={handleAddPlayer} className="whitelist-form">
                <input
                  type="text"
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  placeholder="Minecraft username"
                  className="whitelist-input"
                  disabled={isAdding}
                />
                <Button
                  type="submit"
                  variant="primary"
                  disabled={isAdding || !newUsername.trim()}
                  loading={isAdding}
                  className="btn-sm"
                >
                  Add
                </Button>
              </form>

              {whitelist && whitelist.players.length > 0 ? (
                <div className="space-y-1">
                  {whitelist.players.map((player) => (
                    <div key={player.uuid} className="whitelist-player">
                      <div>
                        <Tooltip content={`UUID: ${player.uuid}`}>
                          <span className="whitelist-player-name">
                            {player.username}
                          </span>
                        </Tooltip>
                        <div className="whitelist-player-meta">
                          Added by {player.addedBy}
                        </div>
                      </div>
                      <Button
                        variant="outline"
                        className="btn-sm"
                        onClick={() => handleRemovePlayer(player)}
                        disabled={removePlayerMutation.isPending}
                        aria-label={`Remove ${player.username}`}
                      >
                        <X className="h-3 w-3" />
                      </Button>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="whitelist-empty">No players in whitelist</p>
              )}
            </>
          )}
        </div>
      )}
    </div>
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
  const { copy, copied } = useCopyToClipboard();

  const isMutating = startMutation.isPending || stopMutation.isPending;

  /**
   * Handles copying the server IP to clipboard with toast feedback
   */
  const handleCopyIP = async (ip: string) => {
    const success = await copy(ip);
    if (success) {
      toast.success('IP copied to clipboard!');
    } else {
      toast.error('Failed to copy IP');
    }
  };

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
          {isRunning && status.ip && (
            <Button variant="outline" onClick={() => handleCopyIP(status.ip)}>
              {copied ? (
                <>
                  <Check className="h-4 w-4" />
                  Copied!
                </>
              ) : (
                <>
                  <Copy className="h-4 w-4" />
                  Copy IP
                </>
              )}
            </Button>
          )}
        </div>

        <WhitelistSection isRunning={isRunning} />
      </CardContent>
    </Card>
  );
}
