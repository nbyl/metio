import { useQuery } from '@tanstack/react-query';
import type { ServerResponse } from '../types/server';

const POLL_INTERVAL_MS = 10000;

async function fetchServers(): Promise<ServerResponse[]> {
  const response = await fetch('/api/servers');

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    throw new Error(`Failed to fetch servers: ${response.status}`);
  }

  return response.json();
}

export function useServers() {
  return useQuery({
    queryKey: ['servers'],
    queryFn: fetchServers,
    refetchInterval: POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    staleTime: 0,
  });
}
