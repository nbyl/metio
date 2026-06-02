import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type {
  APIError,
  StatusResponse,
} from '../types/server';

interface ScheduleShutdownRequest {
  shutdownTime: string;
}

interface ScheduleShutdownResponse {
  success: boolean;
  scheduledShutdown?: string;
}

async function scheduleShutdown(
  serverId: string,
  shutdownTime: string
): Promise<ScheduleShutdownResponse> {
  const response = await fetch(`/api/servers/${serverId}/shutdown/schedule`, {
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

async function cancelScheduledShutdown(
  serverId: string
): Promise<ScheduleShutdownResponse> {
  const response = await fetch(`/api/servers/${serverId}/shutdown/schedule`, {
    method: 'DELETE',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to cancel scheduled shutdown');
  }
  return response.json();
}

export function useScheduleShutdown(serverId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (shutdownTime: string) => scheduleShutdown(serverId, shutdownTime),
    onMutate: async (shutdownTime) => {
      await queryClient.cancelQueries({ queryKey: ['serverStatus', serverId] });

      const previousStatus =
        queryClient.getQueryData<StatusResponse>(['serverStatus', serverId]);

      queryClient.setQueryData<StatusResponse>(['serverStatus', serverId], (old) => {
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
      queryClient.invalidateQueries({ queryKey: ['serverStatus', serverId] });
    },
    onError: (error: Error, _shutdownTime, context) => {
      if (context?.previousStatus) {
        queryClient.setQueryData(['serverStatus', serverId], context.previousStatus);
      }
      toast.error(error.message);
    },
  });
}

export function useCancelScheduledShutdown(serverId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => cancelScheduledShutdown(serverId),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: ['serverStatus', serverId] });

      const previousStatus =
        queryClient.getQueryData<StatusResponse>(['serverStatus', serverId]);

      queryClient.setQueryData<StatusResponse>(['serverStatus', serverId], (old) => {
        if (!old) return old;
        return { ...old, scheduledShutdown: undefined };
      });

      return { previousStatus };
    },
    onSuccess: () => {
      toast.success('Scheduled shutdown cancelled');
      queryClient.invalidateQueries({ queryKey: ['serverStatus', serverId] });
    },
    onError: (error: Error, _vars, context) => {
      if (context?.previousStatus) {
        queryClient.setQueryData(['serverStatus', serverId], context.previousStatus);
      }
      toast.error(error.message);
    },
  });
}
