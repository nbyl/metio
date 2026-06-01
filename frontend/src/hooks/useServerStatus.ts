import { useQuery } from '@tanstack/react-query';
import type { StatusResponse } from '../types/server';

const POLL_INTERVAL_MS = 5000;

async function fetchServerStatus(serverId: string): Promise<StatusResponse> {
  const response = await fetch(`/api/servers/${serverId}/status`);
  if (!response.ok) {
    throw new Error(
      `Failed to fetch status: ${response.status} ${response.statusText}`
    );
  }
  return response.json();
}

export function useServerStatus(serverId: string) {
  return useQuery({
    queryKey: ['serverStatus', serverId],
    queryFn: () => fetchServerStatus(serverId),
    refetchInterval: POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    staleTime: 0,
  });
}
