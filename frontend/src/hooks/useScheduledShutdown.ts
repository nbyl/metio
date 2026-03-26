import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type {
  ScheduleShutdownRequest,
  ScheduleShutdownResponse,
  APIError,
  ServerStatus,
} from '../types/server';

/**
 * Schedules a server shutdown
 */
async function scheduleShutdown(
  shutdownTime: string
): Promise<ScheduleShutdownResponse> {
  const response = await fetch('/api/server/shutdown/schedule', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ shutdownTime } as ScheduleShutdownRequest),
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to schedule shutdown');
  }
  return response.json();
}

/**
 * Cancels a scheduled shutdown
 */
async function cancelScheduledShutdown(): Promise<ScheduleShutdownResponse> {
  const response = await fetch('/api/server/shutdown/schedule', {
    method: 'DELETE',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to cancel scheduled shutdown');
  }
  return response.json();
}

/**
 * Hook to schedule a server shutdown
 */
export function useScheduleShutdown() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: scheduleShutdown,
    onMutate: async (shutdownTime) => {
      // Cancel any outgoing refetches
      await queryClient.cancelQueries({ queryKey: ['serverStatus'] });

      // Snapshot the previous value
      const previousStatus =
        queryClient.getQueryData<ServerStatus>(['serverStatus']);

      // Optimistically update
      queryClient.setQueryData<ServerStatus>(['serverStatus'], (old) => {
        if (!old) return old;
        return { ...old, scheduledShutdown: shutdownTime };
      });

      return { previousStatus };
    },
    onSuccess: (data) => {
      const time = data.scheduledShutdown
        ? new Date(data.scheduledShutdown).toLocaleTimeString()
        : '';
      toast.success(`Shutdown scheduled for ${time}`);
      queryClient.invalidateQueries({ queryKey: ['serverStatus'] });
    },
    onError: (error: Error, _shutdownTime, context) => {
      // Rollback on error
      if (context?.previousStatus) {
        queryClient.setQueryData(['serverStatus'], context.previousStatus);
      }
      toast.error(error.message);
    },
  });
}

/**
 * Hook to cancel a scheduled shutdown
 */
export function useCancelScheduledShutdown() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: cancelScheduledShutdown,
    onMutate: async () => {
      // Cancel any outgoing refetches
      await queryClient.cancelQueries({ queryKey: ['serverStatus'] });

      // Snapshot the previous value
      const previousStatus =
        queryClient.getQueryData<ServerStatus>(['serverStatus']);

      // Optimistically update
      queryClient.setQueryData<ServerStatus>(['serverStatus'], (old) => {
        if (!old) return old;
        return { ...old, scheduledShutdown: undefined };
      });

      return { previousStatus };
    },
    onSuccess: () => {
      toast.success('Scheduled shutdown cancelled');
      queryClient.invalidateQueries({ queryKey: ['serverStatus'] });
    },
    onError: (error: Error, _vars, context) => {
      // Rollback on error
      if (context?.previousStatus) {
        queryClient.setQueryData(['serverStatus'], context.previousStatus);
      }
      toast.error(error.message);
    },
  });
}
