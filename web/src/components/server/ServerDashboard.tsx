import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  Copy,
  Check,
  ChevronDown,
  ChevronRight,
  X,
  Loader2,
  Clock,
  Server,
  Settings,
  Trash2,
} from 'lucide-react';
import { useServers } from '../../hooks/useServers';
import { useServerStatus } from '../../hooks/useServerStatus';
import { useServerProvisioning } from '../../hooks/useServerProvisioning';
import { useStartServer, useStopServer } from '../../hooks/useServerMutations';
import { useUpdateServer, useDeleteServer } from '../../hooks/useServerMutations';
import {
  useWhitelist,
  useAddPlayer,
  useRemovePlayer,
  useToggleWhitelist,
} from '../../hooks/useWhitelist';
import {
  useScheduleShutdown,
  useCancelScheduledShutdown,
} from '../../hooks/useScheduledShutdown';
import { useCopyToClipboard } from '../../hooks/useCopyToClipboard';
import { Card, CardHeader, CardTitle, CardContent } from '../ui/Card';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import { Separator } from '../ui/Separator';
import { Skeleton } from '../ui/Skeleton';
import { Tooltip } from '../ui/Tooltip';
import { Switch } from '../ui/Switch';
import { StatsGrid, type StatItem } from '../layout/StatsGrid';
import { ServerConfigPanel } from './ServerConfigPanel';
import { UpdateModal } from './UpdateModal';
import { DestroyModal } from './DestroyModal';
import { EmptyState } from './EmptyState';
import type { ServerState, UpdateServerRequest, ServerConfig, StatusResponse } from '../../types/server';
import type { WhitelistPlayer } from '../../types/whitelist';
import { cn } from '../../lib/utils';

export interface ServerDashboardProps {
  className?: string;
}

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

function ServerListSkeleton({ className }: { className?: string }) {
  return (
    <div className={cn('space-y-4', className)}>
      {[1, 2].map((i) => (
        <Card key={i}>
          <CardHeader>
            <CardTitle>
              <Skeleton className="h-6 w-48" />
              <Skeleton className="h-5 w-16" />
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="stats-grid">
              {[1, 2, 3, 4].map((j) => (
                <div key={j} className="stat">
                  <Skeleton className="h-4 w-16" />
                  <Skeleton className="h-5 w-20 mt-1" />
                </div>
              ))}
            </div>
            <Separator />
            <div className="controls mt-4">
              <Skeleton className="h-9 w-24" />
              <Skeleton className="h-9 w-24" />
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

interface WhitelistSectionProps {
  serverId: string;
  isRunning: boolean;
}

function WhitelistSection({ serverId, isRunning }: WhitelistSectionProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [newUsername, setNewUsername] = useState('');

  const { data: whitelist, isLoading } = useWhitelist(serverId);
  const addPlayerMutation = useAddPlayer(serverId);
  const removePlayerMutation = useRemovePlayer(serverId);
  const toggleWhitelistMutation = useToggleWhitelist(serverId);

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
                        <Tooltip
                          content={`UUID: ${player.uuid}\nAdded by: ${player.addedBy}`}
                        >
                          <span className="whitelist-player-name">
                            {player.username}
                          </span>
                        </Tooltip>
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

interface ScheduledShutdownSectionProps {
  serverId: string;
  isRunning: boolean;
  scheduledShutdown?: string;
}

function ScheduledShutdownSection({
  serverId,
  isRunning,
  scheduledShutdown,
}: ScheduledShutdownSectionProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [shutdownTime, setShutdownTime] = useState('');

  const scheduleShutdownMutation = useScheduleShutdown(serverId);
  const cancelShutdownMutation = useCancelScheduledShutdown(serverId);

  if (!isRunning) {
    return null;
  }

  const hasScheduledShutdown = !!scheduledShutdown;

  const handleScheduleShutdown = (e: React.FormEvent) => {
    e.preventDefault();
    if (!shutdownTime) return;

    const today = new Date();
    const [hours, minutes] = shutdownTime.split(':').map(Number);
    const scheduledDate = new Date(
      today.getFullYear(),
      today.getMonth(),
      today.getDate(),
      hours,
      minutes,
      0,
      0
    );

    scheduleShutdownMutation.mutate(scheduledDate.toISOString(), {
      onSuccess: () => setShutdownTime(''),
    });
  };

  const handleCancelShutdown = () => {
    cancelShutdownMutation.mutate();
  };

  const formatScheduledTime = (isoString: string): string => {
    const date = new Date(isoString);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  const getTimeUntilShutdown = (isoString: string): string => {
    const shutdownDate = new Date(isoString);
    const now = new Date();
    const diffMs = shutdownDate.getTime() - now.getTime();

    if (diffMs <= 0) return 'imminent';

    const diffMinutes = Math.floor(diffMs / 60000);
    if (diffMinutes < 60) {
      return `${diffMinutes} min`;
    }

    const diffHours = Math.floor(diffMinutes / 60);
    const remainingMinutes = diffMinutes % 60;
    return `${diffHours}h ${remainingMinutes}m`;
  };

  const isScheduling = scheduleShutdownMutation.isPending;
  const isCancelling = cancelShutdownMutation.isPending;

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
          <Clock className="h-4 w-4" />
          Scheduled Shutdown
        </button>
        {hasScheduledShutdown && (
          <Badge variant="transitioning">
            {getTimeUntilShutdown(scheduledShutdown)}
          </Badge>
        )}
      </div>

      {isExpanded && (
        <div>
          {hasScheduledShutdown ? (
            <div className="scheduled-shutdown-active">
              <p className="scheduled-shutdown-info">
                Server will shut down at{' '}
                <strong>{formatScheduledTime(scheduledShutdown)}</strong>
              </p>
              <p className="scheduled-shutdown-warning">
                Players will receive warnings at 5 minutes and 1 minute before
                shutdown.
              </p>
              <Button
                variant="outline"
                onClick={handleCancelShutdown}
                disabled={isCancelling}
                loading={isCancelling}
                className="btn-sm mt-2"
              >
                Cancel Shutdown
              </Button>
            </div>
          ) : (
            <form onSubmit={handleScheduleShutdown} className="whitelist-form">
              <input
                type="time"
                value={shutdownTime}
                onChange={(e) => setShutdownTime(e.target.value)}
                className="whitelist-input"
                disabled={isScheduling}
              />
              <Button
                type="submit"
                variant="danger"
                disabled={isScheduling || !shutdownTime}
                loading={isScheduling}
                className="btn-sm"
              >
                Schedule
              </Button>
            </form>
          )}
        </div>
      )}
    </div>
  );
}

interface ServerCardProps {
  server: {
    id: string;
    config: ServerConfig;
    status?: StatusResponse;
    currentInfraVersion: number;
    outdated: boolean;
  };
}

function ServerCard({ server }: ServerCardProps) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: status } = useServerStatus(server.id);
  const { data: provisioning } = useServerProvisioning(server.id);
  const startMutation = useStartServer(server.id);
  const stopMutation = useStopServer(server.id);
  const updateMutation = useUpdateServer();
  const deleteMutation = useDeleteServer();
  const { copy, copied } = useCopyToClipboard();
  const [showUpdate, setShowUpdate] = useState(false);
  const [showDestroy, setShowDestroy] = useState(false);

  const isMutating = startMutation.isPending || stopMutation.isPending;
  const isProvisioning = provisioning && provisioning.state !== 'COMPLETED' && provisioning.state !== 'FAILED';

  const handleCopyIP = async (ip: string) => {
    const success = await copy(ip);
    if (success) {
      toast.success('IP copied to clipboard!');
    } else {
      toast.error('Failed to copy IP');
    }
  };

  const currentStatus = (status ?? server.status) as StatusResponse | undefined;

  const isRunning = currentStatus?.serverState === 'RUNNING';
  const isStopped = currentStatus?.serverState === 'STOPPED';
  const isTransitioning =
    currentStatus?.serverState === 'STARTING' ||
    currentStatus?.serverState === 'STOPPING';

  const stats: StatItem[] = [
    {
      label: 'State',
      value: currentStatus
        ? getStatusLabel(currentStatus.serverState)
        : 'Unknown',
    },
    {
      label: 'Players',
      value: currentStatus
        ? `${currentStatus.players.current}/${currentStatus.players.max}`
        : '-',
    },
    {
      label: 'Uptime',
      value: currentStatus?.uptime || '-',
    },
    {
      label: 'IP',
      value: currentStatus?.instanceIP || '-',
    },
  ];

  const handleUpdate = (data: UpdateServerRequest) => {
    updateMutation.mutate(
      { id: server.id, data },
      {
        onSuccess: () => {
          queryClient.removeQueries({ queryKey: ['serverProvisioning', server.id] });
          navigate(`/servers/${server.id}/provisioning`, {
            state: { serverName: server.config.name },
          });
        },
      }
    );
  };

  const handleDestroy = (createBackup: boolean) => {
    deleteMutation.mutate(
      { id: server.id, createBackup },
      {
        onSuccess: () => {
          queryClient.removeQueries({ queryKey: ['serverProvisioning', server.id] });
          navigate(`/servers/${server.id}/provisioning`, {
            state: { serverName: server.config.name },
          });
        },
      }
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <span className="flex items-center gap-2">
            <Server className="h-4 w-4" />
            Server: {server.config.name}
            {server.outdated && (
              <span className="ml-2 text-xs text-yellow-400 font-normal">Update Available</span>
            )}
          </span>
          <span className="flex items-center gap-2">
            {currentStatus?.serverState ? (
              <Badge variant={getStatusBadgeVariant(currentStatus.serverState)}>
                {getStatusLabel(currentStatus.serverState)}
              </Badge>
            ) : (
              <Badge variant="offline">Unknown</Badge>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowUpdate(true)}
            >
              <Settings className="h-4 w-4" />
            </Button>
          </span>
        </CardTitle>
      </CardHeader>

      <CardContent>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div>
            <StatsGrid stats={stats} />
          </div>
          <div>
            <ServerConfigPanel
              compact
              config={server.config}
              infrahVersion={server.currentInfraVersion}
              outdated={server.outdated}
            />
          </div>
        </div>

        {isProvisioning && provisioning && (
          <div
            className="mt-4 p-3 -m-3 rounded-lg cursor-pointer hover:bg-slate-700/50 transition-colors"
            onClick={() =>
              navigate(`/servers/${server.id}/provisioning`, {
                state: { serverName: server.config.name },
              })
            }
          >
            <div className="flex items-center justify-between text-xs mb-1.5">
              <span className="text-green-400 font-medium hover:underline">
                {provisioning.operation === 'CREATE' && 'Creating...'}
                {provisioning.operation === 'UPDATE' && 'Updating...'}
                {provisioning.operation === 'DESTROY' && 'Destroying...'}
              </span>
              <span className="flex items-center gap-1 text-slate-400">
                {provisioning.progress}%
                <ChevronRight className="h-3.5 w-3.5" />
              </span>
            </div>
            <div className="h-1.5 rounded-full bg-slate-700 overflow-hidden">
              <div
                className="h-full rounded-full bg-green-500 transition-all duration-500"
                style={{ width: `${provisioning.progress}%` }}
              />
            </div>
          </div>
        )}

        <Separator className="my-4" />

        <div className="flex flex-wrap gap-3">
          {isStopped && (
            <Button
              variant="primary"
              onClick={() => startMutation.mutate()}
              disabled={isMutating || isProvisioning}
              loading={startMutation.isPending}
            >
              Start Server
            </Button>
          )}
          {isTransitioning && (
            <Button variant="outline" disabled>
              {currentStatus?.serverState === 'STARTING'
                ? 'Starting...'
                : 'Stopping...'}
            </Button>
          )}
          {isRunning && (
            <Button
              variant="danger"
              onClick={() => stopMutation.mutate()}
              disabled={isMutating || isProvisioning}
              loading={stopMutation.isPending}
            >
              Stop Server
            </Button>
          )}
          {isRunning && currentStatus?.instanceIP && (
            <Button
              variant="outline"
              disabled={isProvisioning}
              onClick={() => handleCopyIP(currentStatus.instanceIP)}
            >
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
          <Button
            variant={server.outdated ? "primary" : "outline"}
            disabled={isProvisioning}
            onClick={() => setShowUpdate(true)}
          >
            <Settings className="h-4 w-4" />
            Update
          </Button>
          <Button
            variant="danger"
            disabled={isProvisioning}
            onClick={() => setShowDestroy(true)}
          >
            <Trash2 className="h-4 w-4" />
            Destroy
          </Button>
        </div>

        <WhitelistSection serverId={server.id} isRunning={isRunning} />
        <ScheduledShutdownSection
          serverId={server.id}
          isRunning={isRunning}
          scheduledShutdown={currentStatus?.scheduledShutdown}
        />
      </CardContent>

      <UpdateModal
        open={showUpdate}
        config={server.config}
        currentInfraVersion={server.currentInfraVersion}
        outdated={server.outdated}
        onClose={() => setShowUpdate(false)}
        onUpdate={(data) => {
          handleUpdate(data);
          setShowUpdate(false);
        }}
        isPending={updateMutation.isPending}
      />
      <DestroyModal
        open={showDestroy}
        serverName={server.config.name}
        serverState={currentStatus?.serverState}
        onClose={() => setShowDestroy(false)}
        onConfirm={(createBackup) => {
          handleDestroy(createBackup);
          setShowDestroy(false);
        }}
        isPending={deleteMutation.isPending}
      />
    </Card>
  );
}

export function ServerDashboard({ className }: ServerDashboardProps) {
  const navigate = useNavigate();

  const {
    data: servers,
    isLoading,
    error,
    refetch,
  } = useServers();

  if (isLoading) {
    return <ServerListSkeleton className={className} />;
  }

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

  if (!servers || servers.length === 0) {
    return (
      <EmptyState
        className={className}
        onCreateServer={() => navigate('/servers/new')}
      />
    );
  }

  return (
    <div className={cn('space-y-6', className)}>
      {servers.map((server) => (
        <ServerCard key={server.id} server={server} />
      ))}
    </div>
  );
}
