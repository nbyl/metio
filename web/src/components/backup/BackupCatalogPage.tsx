import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ArrowLeft, Plus, RotateCcw, Server, Trash2 } from 'lucide-react';
import type { BackupRecord } from '../../types/server';
import { useAllBackups } from '../../hooks/useBackups';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/Card';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import { Skeleton } from '../ui/Skeleton';
import { Tabs, TabsList, TabsTrigger } from '../ui/Tabs';
import { CreateFromBackupDialog } from './CreateFromBackupDialog';
import { RestoreConfirmDialog } from '../server/RestoreConfirmDialog';

type FilterValue = 'all' | 'active' | 'deleted';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remaining = seconds % 60;
  return remaining > 0 ? `${minutes}m ${remaining}s` : `${minutes}m`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function formatDateShort(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

function LoadingSkeleton() {
  return (
    <div className="space-y-4">
      {[1, 2, 3].map((i) => (
        <Card key={i}>
          <CardContent>
            <div className="flex items-center gap-4">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-5 w-16" />
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-4 w-20" />
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function EmptyState({ onShowAll }: { onShowAll?: () => void }) {
  return (
    <Card>
      <CardContent className="py-12 text-center">
        <Server className="mx-auto h-8 w-8 text-muted-foreground" />
        <p className="mt-3 text-sm text-muted-foreground">
          No backups found. Enable scheduled backups on a server to start
          creating snapshots.
        </p>
        {onShowAll && (
          <Button
            variant="link"
            className="mt-2"
            onClick={onShowAll}
          >
            Show all backups
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

export function BackupCatalogPage() {
  const { data: backups, isLoading, error, refetch } = useAllBackups();
  const [searchParams, setSearchParams] = useSearchParams();
  const serverFilter = searchParams.get('server');
  const [filter, setFilter] = useState<FilterValue>('all');
  const [createFromBackup, setCreateFromBackup] = useState<BackupRecord | null>(
    null
  );
  const [restoreTarget, setRestoreTarget] = useState<BackupRecord | null>(null);

  const filteredBackups = (backups ?? []).filter((backup) => {
    if (serverFilter && backup.serverId !== serverFilter) return false;
    if (filter === 'active') return !backup.serverDeletedAt;
    if (filter === 'deleted') return !!backup.serverDeletedAt;
    return true;
  });

  const filteredServerName = serverFilter
    ? filteredBackups[0]?.serverName ?? null
    : null;

  const handleClearServerFilter = () => {
    setSearchParams({});
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-semibold">Backups</h1>
        <LoadingSkeleton />
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-semibold">Backups</h1>
        <Card>
          <CardContent>
            <p className="text-destructive">Error: {error.message}</p>
            <Button
              variant="outline"
              className="mt-4"
              onClick={() => refetch()}
            >
              Retry
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          {serverFilter && (
            <Button
              variant="ghost"
              size="sm"
              onClick={handleClearServerFilter}
              aria-label="Show all backups"
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
          )}
          <h1 className="text-2xl font-semibold">
            {filteredServerName
              ? `Backups for ${filteredServerName}`
              : 'Backups'}
          </h1>
        </div>
        {!serverFilter && (
          <Tabs
            value={filter}
            onValueChange={(v) => setFilter(v as FilterValue)}
          >
            <TabsList>
              <TabsTrigger value="all">All</TabsTrigger>
              <TabsTrigger value="active">Active</TabsTrigger>
              <TabsTrigger value="deleted">Deleted</TabsTrigger>
            </TabsList>
          </Tabs>
        )}
      </div>

      {filteredBackups.length === 0 ? (
        <EmptyState
          onShowAll={serverFilter ? handleClearServerFilter : undefined}
        />
      ) : (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {filteredBackups.length} backup
              {filteredBackups.length !== 1 ? 's' : ''}
            </CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-muted-foreground">
                    <th className="px-4 py-2 font-medium">Server</th>
                    <th className="px-4 py-2 font-medium">Status</th>
                    <th className="px-4 py-2 font-medium">Created</th>
                    <th className="px-4 py-2 font-medium">Duration</th>
                    <th className="px-4 py-2 font-medium">Files</th>
                    <th className="px-4 py-2 font-medium">Size</th>
                    <th className="px-4 py-2 font-medium">Version</th>
                    <th className="px-4 py-2 font-medium">Retention</th>
                    <th className="px-4 py-2 font-medium text-right">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {filteredBackups.map((backup) => (
                    <tr
                      key={backup.id}
                      className="border-b border-border/50 hover:bg-muted/30"
                    >
                      <td className="px-4 py-2.5">
                        <div className="flex items-center gap-2">
                          <span className="font-medium">
                            {backup.serverName}
                          </span>
                          {backup.serverDeletedAt && (
                            <Badge variant="destructive" className="text-xs">
                              <Trash2 className="mr-1 h-3 w-3" />
                              Deleted
                            </Badge>
                          )}
                        </div>
                        {backup.serverDeletedAt && backup.retentionUntil && (
                          <p className="mt-0.5 text-xs text-muted-foreground">
                            Retention until{' '}
                            {formatDateShort(backup.retentionUntil)}
                          </p>
                        )}
                      </td>
                      <td className="px-4 py-2.5">
                        <Badge
                          variant={
                            backup.status === 'COMPLETED'
                              ? 'default'
                              : 'destructive'
                          }
                        >
                          {backup.status}
                        </Badge>
                      </td>
                      <td className="px-4 py-2.5">
                        {formatDate(backup.createdAt)}
                      </td>
                      <td className="px-4 py-2.5">
                        {formatDuration(backup.durationSeconds)}
                      </td>
                      <td className="px-4 py-2.5">
                        {backup.fileCount.toLocaleString()}
                      </td>
                      <td className="px-4 py-2.5">
                        {formatBytes(backup.repositorySize)}
                      </td>
                      <td className="px-4 py-2.5">{backup.minecraftVersion}</td>
                      <td className="px-4 py-2.5">
                        {backup.retentionUntil ? (
                          <span className="text-muted-foreground">
                            until {formatDateShort(backup.retentionUntil)}
                          </span>
                        ) : (
                          <span className="text-muted-foreground">—</span>
                        )}
                      </td>
                      <td className="px-4 py-2.5 text-right">
                        <div className="flex items-center justify-end gap-2">
                          {backup.status === 'COMPLETED' &&
                            !backup.serverDeletedAt && (
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => setRestoreTarget(backup)}
                              >
                                <RotateCcw className="h-3 w-3" />
                                Restore
                              </Button>
                            )}
                          {backup.status === 'COMPLETED' &&
                            backup.sourceConfig && (
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => setCreateFromBackup(backup)}
                              >
                                <Plus className="h-3 w-3" />
                                Create Server
                              </Button>
                            )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}

      <CreateFromBackupDialog
        open={createFromBackup !== null}
        backup={createFromBackup}
        onClose={() => setCreateFromBackup(null)}
      />

      <RestoreConfirmDialog
        open={restoreTarget !== null}
        backup={restoreTarget}
        serverId={restoreTarget?.serverId ?? ''}
        serverName={restoreTarget?.serverName ?? ''}
        currentMinecraftVersion={restoreTarget?.minecraftVersion}
        onClose={() => setRestoreTarget(null)}
      />
    </div>
  );
}
