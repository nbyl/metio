import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import {
  Copy,
  Check,
  ChevronRight,
  X,
  Loader2,
  Clock,
  Server,
  Settings,
  Trash2,
  Plus,
} from 'lucide-react';
import { useServers } from '../../hooks/useServers';
import { useServerStatus } from '../../hooks/useServerStatus';
import { useServerProvisioning } from '../../hooks/useServerProvisioning';
import { useStartServer, useStopServer } from '../../hooks/useServerMutations';
import {
  useUpdateServer,
  useUpdateAgent,
  useDeleteServer,
} from '../../hooks/useServerMutations';
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '../ui/Tooltip';
import { Switch } from '../ui/Switch';
import { Progress } from '../ui/Progress';
import { Input } from '../ui/Input';
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormMessage,
} from '../ui/Form';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '../ui/Collapsible';
import { StatsGrid, type StatItem } from '../layout/StatsGrid';
import { ServerConfigPanel } from './ServerConfigPanel';
import { UpdateModal } from './UpdateModal';
import { DestroyModal } from './DestroyModal';
import { EmptyState } from './EmptyState';
import type {
  ServerState,
  UpdateServerRequest,
  ServerConfig,
  StatusResponse,
} from '../../types/server';
import type { WhitelistPlayer } from '../../types/whitelist';
import { cn } from '../../lib/utils';

export interface ServerDashboardProps {
  className?: string;
}

function getStatusBadgeVariant(state: ServerState): {
  variant: 'default' | 'destructive' | 'secondary';
  className?: string;
} {
  switch (state) {
    case 'RUNNING':
      return { variant: 'default', className: 'bg-green-600 text-white' };
    case 'STOPPED':
      return { variant: 'destructive' };
    case 'STARTING':
    case 'STOPPING':
      return { variant: 'secondary', className: 'bg-yellow-600 text-white' };
    default:
      return { variant: 'destructive' };
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
            <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
              {[1, 2, 3, 4].map((j) => (
                <div key={j} className="space-y-1">
                  <Skeleton className="h-4 w-16" />
                  <Skeleton className="h-5 w-20 mt-1" />
                </div>
              ))}
            </div>
            <Separator />
            <div className="mt-4 flex gap-3">
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
  const form = useForm<{ username: string }>({
    resolver: zodResolver(
      z.object({ username: z.string().trim().min(1, 'Username is required') })
    ),
    defaultValues: { username: '' },
    mode: 'onTouched',
  });
  const username = useWatch({ control: form.control, name: 'username' });
  const { data: whitelist, isLoading } = useWhitelist(serverId);
  const addPlayerMutation = useAddPlayer(serverId);
  const removePlayerMutation = useRemovePlayer(serverId);
  const toggleWhitelistMutation = useToggleWhitelist(serverId);

  if (!isRunning) {
    return null;
  }

  const handleToggle = (enabled: boolean) => {
    toggleWhitelistMutation.mutate(enabled);
  };

  const handleRemovePlayer = (player: WhitelistPlayer) => {
    removePlayerMutation.mutate(player.uuid);
  };

  const isToggling = toggleWhitelistMutation.isPending;
  const isAdding = addPlayerMutation.isPending;

  return (
    <Collapsible className="border-t border-border pt-3">
      <div className="mb-3 flex items-center justify-between">
        <CollapsibleTrigger className="flex items-center gap-2 text-sm font-medium text-foreground">
          Whitelist{' '}
          {whitelist && (
            <span className="text-muted-foreground">
              ({whitelist.players.length})
            </span>
          )}
        </CollapsibleTrigger>
        {whitelist && (
          <Switch
            checked={whitelist.enabled}
            onCheckedChange={handleToggle}
            disabled={isToggling}
            aria-label="Toggle whitelist"
          />
        )}
      </div>

      <CollapsibleContent>
        <div>
          {isLoading ? (
            <div className="flex items-center justify-center py-4">
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <>
              <Form {...form}>
                <form
                  onSubmit={form.handleSubmit(({ username }) =>
                    addPlayerMutation.mutate(username.trim(), {
                      onSuccess: () => form.reset(),
                    })
                  )}
                  className="mb-3 flex gap-2"
                >
                  <FormField
                    control={form.control}
                    name="username"
                    render={({ field }) => (
                      <FormItem className="flex-1">
                        <FormControl>
                          <Input
                            {...field}
                            placeholder="Minecraft username"
                            disabled={isAdding}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <Button
                    type="submit"
                    variant="default"
                    disabled={isAdding || !username.trim()}
                    size="sm"
                  >
                    {isAdding && <Loader2 className="animate-spin" />}
                    Add
                  </Button>
                </form>
              </Form>

              {whitelist && whitelist.players.length > 0 ? (
                <div className="space-y-1">
                  {whitelist.players.map((player) => (
                    <div
                      key={player.uuid}
                      className="flex items-center justify-between rounded px-2 py-1.5 hover:bg-muted/50"
                    >
                      <div>
                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span className="cursor-help text-sm text-foreground">
                                {player.username}
                              </span>
                            </TooltipTrigger>
                            <TooltipContent className="whitespace-pre-line">
                              {`UUID: ${player.uuid}\nAdded by: ${player.addedBy}`}
                            </TooltipContent>
                          </Tooltip>
                        </TooltipProvider>
                      </div>
                      <Button
                        variant="outline"
                        size="sm"
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
                <p className="py-2 text-center text-sm text-muted-foreground">
                  No players in whitelist
                </p>
              )}
            </>
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
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
  const form = useForm<{ shutdownTime: string }>({
    resolver: zodResolver(
      z.object({ shutdownTime: z.string().min(1, 'Shutdown time is required') })
    ),
    defaultValues: { shutdownTime: '' },
    mode: 'onTouched',
  });
  const shutdownTime = useWatch({
    control: form.control,
    name: 'shutdownTime',
  });
  const scheduleShutdownMutation = useScheduleShutdown(serverId);
  const cancelShutdownMutation = useCancelScheduledShutdown(serverId);

  if (!isRunning) {
    return null;
  }

  const hasScheduledShutdown = !!scheduledShutdown;

  const handleScheduleShutdown = ({
    shutdownTime,
  }: {
    shutdownTime: string;
  }) => {
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
      onSuccess: () => form.reset(),
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
    <Collapsible className="border-t border-border pt-3">
      <div className="mb-3 flex items-center justify-between">
        <CollapsibleTrigger className="flex items-center gap-2 text-sm font-medium text-foreground">
          <Clock className="h-4 w-4" />
          Scheduled Shutdown
        </CollapsibleTrigger>
        {hasScheduledShutdown && (
          <Badge variant="secondary" className="bg-yellow-600 text-white">
            {getTimeUntilShutdown(scheduledShutdown)}
          </Badge>
        )}
      </div>

      <CollapsibleContent>
        <div>
          {hasScheduledShutdown ? (
            <div className="py-2">
              <p className="text-sm text-foreground">
                Server will shut down at{' '}
                <strong>{formatScheduledTime(scheduledShutdown)}</strong>
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                Players will receive warnings at 5 minutes and 1 minute before
                shutdown.
              </p>
              <Button
                variant="outline"
                onClick={handleCancelShutdown}
                disabled={isCancelling}
                size="sm"
                className="mt-2"
              >
                {isCancelling && <Loader2 className="animate-spin" />}
                Cancel Shutdown
              </Button>
            </div>
          ) : (
            <Form {...form}>
              <form
                onSubmit={form.handleSubmit(handleScheduleShutdown)}
                className="flex gap-2"
              >
                <FormField
                  control={form.control}
                  name="shutdownTime"
                  render={({ field }) => (
                    <FormItem className="flex-1">
                      <FormControl>
                        <Input {...field} type="time" disabled={isScheduling} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <Button
                  type="submit"
                  variant="destructive"
                  disabled={isScheduling || !shutdownTime}
                  size="sm"
                >
                  {isScheduling && <Loader2 className="animate-spin" />}
                  Schedule
                </Button>
              </form>
            </Form>
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

interface ServerCardProps {
  server: {
    id: string;
    config: ServerConfig;
    status?: StatusResponse;
    currentInfraVersion: number;
    outdated: boolean;
    outdatedMachineAgent?: boolean;
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
  const updateAgentMutation = useUpdateAgent(server.id);
  const { copy, copied } = useCopyToClipboard();
  const [showUpdate, setShowUpdate] = useState(false);
  const [showDestroy, setShowDestroy] = useState(false);

  const isMutating = startMutation.isPending || stopMutation.isPending;
  const isProvisioning =
    provisioning &&
    provisioning.state !== 'COMPLETED' &&
    provisioning.state !== 'FAILED';

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
    ...(isRunning
      ? [
          {
            label: 'Players',
            value: `${currentStatus!.players.current}/${currentStatus!.players.max}`,
          },
          {
            label: 'Uptime',
            value: currentStatus?.uptime || '-',
          },
        ]
      : []),
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
          queryClient.removeQueries({
            queryKey: ['serverProvisioning', server.id],
          });
          navigate(`/servers/${server.id}/provisioning`, {
            state: { serverName: server.config.name },
          });
        },
      }
    );
  };

  const handleDestroy = () => {
    deleteMutation.mutate(
      { id: server.id },
      {
        onSuccess: () => {
          queryClient.removeQueries({
            queryKey: ['serverProvisioning', server.id],
          });
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
              <span className="ml-2 text-xs text-yellow-400 font-normal">
                Update Available
              </span>
            )}
            {server.outdatedMachineAgent && (
              <span className="text-xs text-orange-400 font-normal">
                Agent: outdated
              </span>
            )}
          </span>
          <span className="flex items-center gap-2">
            {currentStatus?.serverState ? (
              <Badge {...getStatusBadgeVariant(currentStatus.serverState)}>
                {getStatusLabel(currentStatus.serverState)}
              </Badge>
            ) : (
              <Badge variant="destructive">Unknown</Badge>
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
            className="mt-4 p-3 -m-3 rounded-lg cursor-pointer hover:bg-muted transition-colors"
            onClick={() =>
              navigate(`/servers/${server.id}/provisioning`, {
                state: { serverName: server.config.name },
              })
            }
          >
            <div className="flex items-center justify-between text-xs mb-1.5">
              <span className="text-green-600 font-medium hover:underline">
                {provisioning.operation === 'CREATE' && 'Creating...'}
                {provisioning.operation === 'UPDATE' && 'Updating...'}
                {provisioning.operation === 'DESTROY' && 'Destroying...'}
              </span>
              <span className="flex items-center gap-1 text-muted-foreground">
                {provisioning.progress}%
                <ChevronRight className="h-3.5 w-3.5" />
              </span>
            </div>
            <Progress
              value={provisioning.progress}
              aria-valuenow={provisioning.progress}
              aria-label={`${provisioning.operation.toLowerCase()} progress`}
              className="h-1.5 [&>[data-slot=progress-indicator]]:bg-green-500"
            />
          </div>
        )}

        <Separator className="my-4" />

        <div className="flex flex-wrap gap-3">
          {isStopped && (
            <Button
              variant="default"
              onClick={() => startMutation.mutate()}
              disabled={isMutating || isProvisioning}
            >
              {startMutation.isPending && <Loader2 className="animate-spin" />}
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
              variant="destructive"
              onClick={() => stopMutation.mutate()}
              disabled={isMutating || isProvisioning}
            >
              {stopMutation.isPending && <Loader2 className="animate-spin" />}
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
            variant={server.outdated ? 'default' : 'outline'}
            disabled={isProvisioning}
            onClick={() => setShowUpdate(true)}
          >
            <Settings className="h-4 w-4" />
            Update
          </Button>
          {server.outdatedMachineAgent && (
            <Button
              variant="default"
              disabled={isProvisioning || updateAgentMutation.isPending}
              onClick={() => {
                updateAgentMutation.mutate(undefined, {
                  onSuccess: () => {
                    queryClient.removeQueries({
                      queryKey: ['serverProvisioning', server.id],
                    });
                    navigate(`/servers/${server.id}/provisioning`, {
                      state: { serverName: server.config.name },
                    });
                  },
                });
              }}
            >
              {updateAgentMutation.isPending && (
                <Loader2 className="animate-spin" />
              )}
              Update Agent
            </Button>
          )}
          <Button
            variant="destructive"
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
        serverId={server.id}
        serverName={server.config.name}
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
        onClose={() => setShowDestroy(false)}
        onConfirm={() => {
          handleDestroy();
          setShowDestroy(false);
        }}
        isPending={deleteMutation.isPending}
      />
    </Card>
  );
}

export function ServerDashboard({ className }: ServerDashboardProps) {
  const navigate = useNavigate();

  const { data: servers, isLoading, error, refetch } = useServers();

  if (isLoading) {
    return <ServerListSkeleton className={className} />;
  }

  if (error) {
    return (
      <Card className={className}>
        <CardContent>
          <p className="text-destructive">Error: {error.message}</p>
          <div className="mt-4 flex gap-3">
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
      <div className="flex justify-end">
        <Button variant="default" onClick={() => navigate('/servers/new')}>
          <Plus className="h-4 w-4" />
          Create Server
        </Button>
      </div>
      {servers.map((server) => (
        <ServerCard key={server.id} server={server} />
      ))}
    </div>
  );
}
