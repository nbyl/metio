import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type {
  StatusResponse,
  ServerResponse,
  UpdateServerRequest,
  CreateServerRequest,
  APIError,
} from '../types/server';

interface ServerActionResponse {
  success: boolean;
  state: string;
}

interface MutationContext {
  previousStatus: StatusResponse | undefined;
}

async function startServer(serverId: string): Promise<ServerActionResponse> {
  const response = await fetch(`/api/servers/${serverId}/start`, { method: 'POST' });

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to start server');
  }

  return response.json();
}

async function stopServer(serverId: string): Promise<ServerActionResponse> {
  const response = await fetch(`/api/servers/${serverId}/stop`, { method: 'POST' });

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to stop server');
  }

  return response.json();
}

export function useCreateServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: CreateServerRequest): Promise<{ id: string }> => {
      const response = await fetch('/api/servers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });

      if (response.status === 401) {
        window.location.href = '/auth/login';
        throw new Error('Session expired');
      }

      if (!response.ok) {
        const err: APIError = await response.json();
        throw new Error(err.error || 'Failed to create server');
      }

      return response.json();
    },
    onSuccess: () => {
      toast.success('Server creation started');
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });
}

export function useStartServer(serverId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => startServer(serverId),
    onMutate: async (): Promise<MutationContext> => {
      await queryClient.cancelQueries({ queryKey: ['serverStatus', serverId] });

      const previousStatus = queryClient.getQueryData<StatusResponse>(['serverStatus', serverId]);

      if (previousStatus) {
        queryClient.setQueryData<StatusResponse>(['serverStatus', serverId], {
          ...previousStatus,
          serverState: 'STARTING',
        });
      }

      return { previousStatus };
    },
    onSuccess: () => {
      toast.success('Server starting...');
      queryClient.invalidateQueries({ queryKey: ['serverStatus', serverId] });
    },
    onError: (error: Error, _variables: void, context: MutationContext | undefined) => {
      if (context?.previousStatus) {
        queryClient.setQueryData(['serverStatus', serverId], context.previousStatus);
      }
      toast.error(error.message);
    },
  });
}

export function useUpdateServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      id,
      data,
    }: {
      id: string;
      data: UpdateServerRequest;
    }): Promise<ServerResponse> => {
      const response = await fetch(`/api/servers/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });

      if (response.status === 401) {
        window.location.href = '/auth/login';
        throw new Error('Session expired');
      }

      if (!response.ok) {
        const err: APIError = await response.json();
        throw new Error(err.error || 'Failed to update server');
      }

      return response.json();
    },
    onSuccess: () => {
      toast.success('Server update started');
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });
}

export function useDeleteServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      id,
      createBackup,
    }: {
      id: string;
      createBackup: boolean;
    }): Promise<void> => {
      const params = createBackup ? '?backup=true' : '';
      const response = await fetch(`/api/servers/${id}${params}`, {
        method: 'DELETE',
      });

      if (response.status === 401) {
        window.location.href = '/auth/login';
        throw new Error('Session expired');
      }

      if (!response.ok) {
        const err: APIError = await response.json();
        throw new Error(err.error || 'Failed to delete server');
      }
    },
    onSuccess: () => {
      toast.success('Server deletion started');
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });
}

export function useStopServer(serverId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => stopServer(serverId),
    onMutate: async (): Promise<MutationContext> => {
      await queryClient.cancelQueries({ queryKey: ['serverStatus', serverId] });

      const previousStatus = queryClient.getQueryData<StatusResponse>(['serverStatus', serverId]);

      if (previousStatus) {
        queryClient.setQueryData<StatusResponse>(['serverStatus', serverId], {
          ...previousStatus,
          serverState: 'STOPPING',
        });
      }

      return { previousStatus };
    },
    onSuccess: () => {
      toast.success('Server stopping...');
      queryClient.invalidateQueries({ queryKey: ['serverStatus', serverId] });
    },
    onError: (error: Error, _variables: void, context: MutationContext | undefined) => {
      if (context?.previousStatus) {
        queryClient.setQueryData(['serverStatus', serverId], context.previousStatus);
      }
      toast.error(error.message);
    },
  });
}
