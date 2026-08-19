import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { APIError, BackupSettings } from '../types/server';

const BACKUP_SETTINGS_QUERY_KEY = 'backupSettings';

async function fetchBackupSettings(serverId: string): Promise<BackupSettings> {
  const response = await fetch(`/api/servers/${serverId}/settings/backup`);

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to fetch backup settings');
  }

  return response.json();
}

export function useBackupSettings(serverId: string) {
  return useQuery({
    queryKey: [BACKUP_SETTINGS_QUERY_KEY, serverId],
    queryFn: () => fetchBackupSettings(serverId),
    staleTime: 0,
  });
}

async function updateBackupSettings(
  serverId: string,
  settings: BackupSettings
): Promise<BackupSettings> {
  const response = await fetch(`/api/servers/${serverId}/settings/backup`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  });

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to update backup settings');
  }

  return response.json();
}

export function useUpdateBackupSettings(serverId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (settings: BackupSettings) =>
      updateBackupSettings(serverId, settings),
    onSuccess: () => {
      toast.success('Backup settings saved. Re-provisioning server...');
      queryClient.invalidateQueries({
        queryKey: [BACKUP_SETTINGS_QUERY_KEY, serverId],
      });
      queryClient.removeQueries({
        queryKey: ['serverProvisioning', serverId],
      });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });
}
