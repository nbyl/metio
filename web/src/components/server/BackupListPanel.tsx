import { useState } from 'react';
import { Loader2, RotateCcw } from 'lucide-react';
import type { BackupRecord } from '../../types/server';
import { useServerBackups } from '../../hooks/useBackups';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import { RestoreConfirmDialog } from './RestoreConfirmDialog';

export interface BackupListPanelProps {
  serverId: string;
  serverName: string;
  minecraftVersion: string;
}

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
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function BackupListPanel({
  serverId,
  serverName,
  minecraftVersion,
}: BackupListPanelProps) {
  const { data: backups, isLoading } = useServerBackups(serverId);
  const [restoreBackup, setRestoreBackup] = useState<BackupRecord | null>(null);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!backups || backups.length === 0) {
    return (
      <p className="py-4 text-center text-sm text-muted-foreground">
        No backups yet. Enable scheduled backups in the Backup tab to start
        creating snapshots.
      </p>
    );
  }

  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left text-muted-foreground">
              <th className="pb-2 font-medium">Created</th>
              <th className="pb-2 font-medium">Duration</th>
              <th className="pb-2 font-medium">Files</th>
              <th className="pb-2 font-medium">Size</th>
              <th className="pb-2 font-medium">Version</th>
              <th className="pb-2 font-medium text-right">Action</th>
            </tr>
          </thead>
          <tbody>
            {backups.map((backup) => (
              <tr key={backup.id} className="border-b border-border/50">
                <td className="py-2">{formatDate(backup.createdAt)}</td>
                <td className="py-2">
                  {formatDuration(backup.durationSeconds)}
                </td>
                <td className="py-2">{backup.fileCount.toLocaleString()}</td>
                <td className="py-2">{formatBytes(backup.repositorySize)}</td>
                <td className="py-2">
                  <Badge
                    variant={
                      backup.minecraftVersion === minecraftVersion
                        ? 'secondary'
                        : 'outline'
                    }
                  >
                    {backup.minecraftVersion}
                  </Badge>
                </td>
                <td className="py-2 text-right">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={backup.status !== 'COMPLETED'}
                    onClick={() => setRestoreBackup(backup)}
                  >
                    <RotateCcw className="h-3 w-3" />
                    Restore
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <RestoreConfirmDialog
        open={restoreBackup !== null}
        backup={restoreBackup}
        serverId={serverId}
        serverName={serverName}
        currentMinecraftVersion={minecraftVersion}
        onClose={() => setRestoreBackup(null)}
      />
    </div>
  );
}
