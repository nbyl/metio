import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type {
  ServerActionResponse,
  ServerStatus,
  ServerResponse,
  UpdateServerRequest,
  APIError,
} from '../types/server';

/** Context for mutation rollback */
interface MutationContext {
  previousStatus: ServerStatus | undefined;
}

/**
 * Sends a POST request to start the server
 */
async function startServer(): Promise<ServerActionResponse> {
  const response = await fetch('/api/server/start', { method: 'POST' });

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

/**
 * Sends a POST request to stop the server
 */
async function stopServer(): Promise<ServerActionResponse> {
  const response = await fetch('/api/server/stop', { method: 'POST' });

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

/**
 * Mutation hook for starting the server
 *
 * @returns React Query mutation for starting the server
 *
 * @example
 * ```tsx
 * const startMutation = useStartServer();
 *
 * <button
 *   onClick={() => startMutation.mutate()}
 *   disabled={startMutation.isPending}
 * >
 *   {startMutation.isPending ? 'Starting...' : 'Start Server'}
 * </button>
 * ```
 */
export function useStartServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: startServer,
    onMutate: async (): Promise<MutationContext> => {
      // Cancel outgoing refetches to prevent race conditions
      await queryClient.cancelQueries({ queryKey: ['serverStatus'] });

      // Snapshot previous value for rollback
      const previousStatus = queryClient.getQueryData<ServerStatus>(['serverStatus']);

      // Optimistically update to STARTING
      if (previousStatus) {
        queryClient.setQueryData<ServerStatus>(['serverStatus'], {
          ...previousStatus,
          status: 'STARTING',
        });
      }

      return { previousStatus };
    },
    onSuccess: () => {
      toast.success('Server starting...');
      queryClient.invalidateQueries({ queryKey: ['serverStatus'] });
    },
    onError: (error: Error, _variables: void, context: MutationContext | undefined) => {
      // Rollback on error
      if (context?.previousStatus) {
        queryClient.setQueryData(['serverStatus'], context.previousStatus);
      }
      toast.error(error.message);
    },
  });
}

/**
 * Mutation hook for stopping the server
 *
 * @returns React Query mutation for stopping the server
 *
 * @example
 * ```tsx
 * const stopMutation = useStopServer();
 *
 * <button
 *   onClick={() => stopMutation.mutate()}
 *   disabled={stopMutation.isPending}
 * >
 *   {stopMutation.isPending ? 'Stopping...' : 'Stop Server'}
 * </button>
 * ```
 */
/**
 * Mutation hook for updating a server via PUT /api/servers/{id}
 */
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

/**
 * Mutation hook for deleting a server via DELETE /api/servers/{id}
 */
export function useDeleteServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string): Promise<void> => {
      const response = await fetch(`/api/servers/${id}`, {
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

export function useStopServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: stopServer,
    onMutate: async (): Promise<MutationContext> => {
      // Cancel outgoing refetches to prevent race conditions
      await queryClient.cancelQueries({ queryKey: ['serverStatus'] });

      // Snapshot previous value for rollback
      const previousStatus = queryClient.getQueryData<ServerStatus>(['serverStatus']);

      // Optimistically update to STOPPING
      if (previousStatus) {
        queryClient.setQueryData<ServerStatus>(['serverStatus'], {
          ...previousStatus,
          status: 'STOPPING',
        });
      }

      return { previousStatus };
    },
    onSuccess: () => {
      toast.success('Server stopping...');
      queryClient.invalidateQueries({ queryKey: ['serverStatus'] });
    },
    onError: (error: Error, _variables: void, context: MutationContext | undefined) => {
      // Rollback on error
      if (context?.previousStatus) {
        queryClient.setQueryData(['serverStatus'], context.previousStatus);
      }
      toast.error(error.message);
    },
  });
}
