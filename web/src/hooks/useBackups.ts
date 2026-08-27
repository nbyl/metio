import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type {
  APIError,
  BackupRecord,
  CreateFromBackupRequest,
  PaginatedBackupsResponse,
  RestoreResponse,
} from '../types/server';

const ALL_BACKUPS_KEY = 'allBackups';
const SERVER_BACKUPS_KEY = 'serverBackups';

export interface BackupQueryParams {
  sort?: string;
  dir?: string;
  limit?: number;
  offset?: number;
  server?: string;
}

async function fetchAllBackups(
  params: BackupQueryParams
): Promise<PaginatedBackupsResponse> {
  const query = new URLSearchParams();
  if (params.sort) query.set('sort', params.sort);
  if (params.dir) query.set('dir', params.dir);
  if (params.limit != null) query.set('limit', String(params.limit));
  if (params.offset != null) query.set('offset', String(params.offset));
  if (params.server) query.set('server', params.server);

  const qs = query.toString();
  const url = `/api/backups${qs ? `?${qs}` : ''}`;

  const response = await fetch(url);

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to fetch backups');
  }

  return response.json();
}

export function useAllBackups(params: BackupQueryParams = {}) {
  return useQuery({
    queryKey: [ALL_BACKUPS_KEY, params],
    queryFn: () => fetchAllBackups(params),
    staleTime: 0,
  });
}

async function fetchServerBackups(serverId: string): Promise<BackupRecord[]> {
  const response = await fetch(`/api/servers/${serverId}/backups`);

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to fetch backups');
  }

  return response.json();
}

export function useServerBackups(serverId: string) {
  return useQuery({
    queryKey: [SERVER_BACKUPS_KEY, serverId],
    queryFn: () => fetchServerBackups(serverId),
    staleTime: 0,
  });
}

async function restoreBackup(
  serverId: string,
  backupId: string
): Promise<RestoreResponse> {
  const response = await fetch(
    `/api/servers/${serverId}/backups/${backupId}/restore`,
    { method: 'POST' }
  );

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to restore backup');
  }

  return response.json();
}

export function useRestoreBackup(serverId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (backupId: string) => restoreBackup(serverId, backupId),
    onSuccess: () => {
      toast.success('Restore started');
      queryClient.invalidateQueries({
        queryKey: [SERVER_BACKUPS_KEY, serverId],
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

async function createServerFromBackup(
  backupId: string,
  request: CreateFromBackupRequest
): Promise<{ id: string }> {
  const response = await fetch(`/api/backups/${backupId}/servers`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to create server from backup');
  }

  return response.json();
}

export function useCreateServerFromBackup(backupId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: CreateFromBackupRequest) =>
      createServerFromBackup(backupId, request),
    onSuccess: () => {
      toast.success('Server creation started');
      queryClient.invalidateQueries({ queryKey: [ALL_BACKUPS_KEY] });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });
}
