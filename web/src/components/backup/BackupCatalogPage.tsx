import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  ChevronLeft,
  ChevronRight,
  MoreHorizontal,
  Plus,
  RotateCcw,
  Server,
  Trash2,
} from 'lucide-react';
import type { BackupRecord } from '../../types/server';
import { useAllBackups } from '../../hooks/useBackups';
import { useServers } from '../../hooks/useServers';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/Card';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import { Skeleton } from '../ui/Skeleton';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from '../ui/DropdownMenu';
import { CreateFromBackupDialog } from './CreateFromBackupDialog';
import { RestoreConfirmDialog } from '../server/RestoreConfirmDialog';

type FilterValue = 'all' | 'active' | 'deleted';
type SortField = 'created_at' | 'duration_seconds' | 'repository_size';
type SortDir = 'asc' | 'desc';

const PAGE_SIZE_OPTIONS = [25, 50, 100, 'All'] as const;

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

interface SortHeaderProps {
  field: SortField;
  label: string;
  sortField: SortField;
  sortDir: SortDir;
  onSort: (field: SortField) => void;
}

function SortHeader({ field, label, sortField, sortDir, onSort }: SortHeaderProps) {
  const active = sortField === field;
  return (
    <th
      className="px-4 py-2 font-medium cursor-pointer select-none hover:text-foreground"
      onClick={() => onSort(field)}
    >
      <span className="inline-flex items-center gap-1">
        {label}
        {active ? (
          sortDir === 'asc' ? (
            <ArrowUp className="h-3 w-3" />
          ) : (
            <ArrowDown className="h-3 w-3" />
          )
        ) : (
          <ArrowDown className="h-3 w-3 opacity-30" />
        )}
      </span>
    </th>
  );
}

export function BackupCatalogPage() {
  const { data: servers } = useServers();
  const [searchParams, setSearchParams] = useSearchParams();

  const serverFilter = searchParams.get('server') ?? '';
  const sortField = (searchParams.get('sort') ?? 'created_at') as SortField;
  const sortDir = (searchParams.get('dir') ?? 'desc') as SortDir;
  const page = Math.max(1, parseInt(searchParams.get('page') ?? '1', 10) || 1);
  const pageSizeParam = searchParams.get('pageSize');
  const pageSize = pageSizeParam === 'All' ? 1000 : (parseInt(pageSizeParam ?? '25', 10) || 25);
  const [filter, setFilter] = useState<FilterValue>('all');
  const [createFromBackup, setCreateFromBackup] = useState<BackupRecord | null>(null);
  const [restoreTarget, setRestoreTarget] = useState<BackupRecord | null>(null);

  const limit = pageSize;
  const offset = (page - 1) * limit;

  const { data, isLoading, error, refetch } = useAllBackups({
    sort: sortField,
    dir: sortDir,
    limit,
    offset,
    server: serverFilter || undefined,
  });

  const total = data?.total ?? 0;
  const backups = data?.backups ?? [];
  const totalPages = pageSize >= 1000 ? 1 : Math.max(1, Math.ceil(total / pageSize));

  const updateParam = (key: string, value: string | null) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (value === null) {
        next.delete(key);
      } else {
        next.set(key, value);
      }
      if (key !== 'page') {
        next.set('page', '1');
      }
      return next;
    });
  };

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      updateParam('dir', sortDir === 'asc' ? 'desc' : 'asc');
    } else {
      updateParam('sort', field);
      updateParam('dir', 'desc');
    }
  };

  const handleServerFilter = (serverId: string) => {
    updateParam('server', serverId || null);
  };

  const handlePageSizeChange = (size: number | 'All') => {
    updateParam('pageSize', size === 'All' ? 'All' : String(size));
  };

  const handleClearServerFilter = () => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      next.delete('server');
      next.delete('page');
      return next;
    });
  };

  const filteredServerName = serverFilter
    ? backups.find((b) => b.serverId === serverFilter)?.serverName ?? null
    : null;

  const filteredByStatus = backups.filter((backup) => {
    if (filter === 'active') return !backup.serverDeletedAt;
    if (filter === 'deleted') return !!backup.serverDeletedAt;
    return true;
  });

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
        <div className="flex items-center gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="sm">
                <Server className="mr-1 h-3.5 w-3.5" />
                {serverFilter
                  ? (servers?.find((s) => s.id === serverFilter)?.config.name ?? 'Filtered')
                  : 'All Servers'}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel>Filter by server</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => handleServerFilter('')}>
                All Servers
              </DropdownMenuItem>
              {servers?.map((s) => (
                <DropdownMenuItem
                  key={s.id}
                  onClick={() => handleServerFilter(s.id)}
                >
                  {s.config.name}
                  {s.id === serverFilter && ' (selected)'}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {filteredByStatus.length === 0 && !serverFilter ? (
        <EmptyState />
      ) : (
        <>
          <Card>
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  {total} backup{total !== 1 ? 's' : ''}
                </CardTitle>
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">Status:</span>
                  {(['all', 'active', 'deleted'] as FilterValue[]).map((v) => (
                    <Badge
                      key={v}
                      variant={filter === v ? 'default' : 'outline'}
                      className="cursor-pointer"
                      onClick={() => setFilter(v)}
                    >
                      {v.charAt(0).toUpperCase() + v.slice(1)}
                    </Badge>
                  ))}
                </div>
              </div>
            </CardHeader>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-muted-foreground">
                      <th className="px-4 py-2 font-medium">Server</th>
                      <th className="px-4 py-2 font-medium">Status</th>
                      <SortHeader
                        field="created_at"
                        label="Created"
                        sortField={sortField}
                        sortDir={sortDir}
                        onSort={handleSort}
                      />
                      <SortHeader
                        field="duration_seconds"
                        label="Duration"
                        sortField={sortField}
                        sortDir={sortDir}
                        onSort={handleSort}
                      />
                      <th className="px-4 py-2 font-medium">Files</th>
                      <SortHeader
                        field="repository_size"
                        label="Size"
                        sortField={sortField}
                        sortDir={sortDir}
                        onSort={handleSort}
                      />
                      <th className="px-4 py-2 font-medium">Version</th>
                      <th className="px-4 py-2 font-medium">Retention</th>
                      <th className="px-4 py-2 font-medium text-right">
                        Actions
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredByStatus.map((backup) => (
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
                          {(backup.status === 'COMPLETED' && !backup.serverDeletedAt) ||
                          (backup.status === 'COMPLETED' && backup.sourceConfig) ? (
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="icon-sm">
                                  <MoreHorizontal className="h-4 w-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                {backup.status === 'COMPLETED' &&
                                  !backup.serverDeletedAt && (
                                    <DropdownMenuItem
                                      onClick={() => setRestoreTarget(backup)}
                                    >
                                      <RotateCcw className="h-4 w-4" />
                                      Restore
                                    </DropdownMenuItem>
                                  )}
                                {backup.status === 'COMPLETED' &&
                                  backup.sourceConfig && (
                                    <DropdownMenuItem
                                      onClick={() => setCreateFromBackup(backup)}
                                    >
                                      <Plus className="h-4 w-4" />
                                      Create Server
                                    </DropdownMenuItem>
                                  )}
                              </DropdownMenuContent>
                            </DropdownMenu>
                          ) : null}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>

          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">Page size:</span>
              {PAGE_SIZE_OPTIONS.map((size) => (
                <Badge
                  key={size}
                  variant={
                    (pageSizeParam ?? '25') === String(size) ||
                    (size === 'All' && pageSizeParam === 'All')
                      ? 'default'
                      : 'outline'
                  }
                  className="cursor-pointer"
                  onClick={() => handlePageSizeChange(size)}
                >
                  {size}
                </Badge>
              ))}
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">
                {offset + 1}–{Math.min(offset + limit, total)} of {total}
              </span>
              <Button
                variant="outline"
                size="icon-sm"
                disabled={page <= 1}
                onClick={() => updateParam('page', String(page - 1))}
                aria-label="Previous page"
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <Button
                variant="outline"
                size="icon-sm"
                disabled={page >= totalPages}
                onClick={() => updateParam('page', String(page + 1))}
                aria-label="Next page"
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </>
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
